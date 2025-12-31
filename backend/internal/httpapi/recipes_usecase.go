package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

// recipeWorkflows captures the DB boundary needed by the recipes use-cases while
// remaining mockable for fast unit tests.
type recipeWorkflows interface {
	CountTagsByIDs(ctx context.Context, ids []pgtype.UUID) (int32, error)
	WithinTx(ctx context.Context, fn func(q recipeWorkflowQueries) error) error
}

// recipeWorkflowQueries are the query methods used inside a transaction for
// recipe create/update workflows.
type recipeWorkflowQueries interface {
	CreateRecipe(ctx context.Context, arg sqlc.CreateRecipeParams) (sqlc.Recipe, error)
	CreateRecipeIngredient(ctx context.Context, arg sqlc.CreateRecipeIngredientParams) error
	CreateRecipeStep(ctx context.Context, arg sqlc.CreateRecipeStepParams) error
	CreateRecipeTag(ctx context.Context, arg sqlc.CreateRecipeTagParams) error

	GetItemByID(ctx context.Context, id pgtype.UUID) (sqlc.GetItemByIDRow, error)
	GetItemByName(ctx context.Context, name string) (sqlc.Item, error)
	CreateItem(ctx context.Context, arg sqlc.CreateItemParams) (sqlc.Item, error)

	GetRecipeDeletedAtByID(ctx context.Context, id pgtype.UUID) (pgtype.Timestamptz, error)
	UpdateRecipeByID(ctx context.Context, arg sqlc.UpdateRecipeByIDParams) (sqlc.Recipe, error)
	DeleteRecipeIngredientsByRecipeID(ctx context.Context, recipeID pgtype.UUID) error
	DeleteRecipeStepsByRecipeID(ctx context.Context, recipeID pgtype.UUID) error
	DeleteRecipeTagsByRecipeID(ctx context.Context, recipeID pgtype.UUID) error
}

// recipeValidationError is returned by recipes use-cases when the request is
// well-formed JSON but fails domain validation at a boundary (UUID parsing,
// reference existence, numeric conversion, etc.).
type recipeValidationError struct {
	FieldErrors []response.FieldError
}

func (e *recipeValidationError) Error() string {
	return "recipe validation failed"
}

func recipeValidationField(field, message string) error {
	return &recipeValidationError{FieldErrors: []response.FieldError{{Field: field, Message: message}}}
}

// recipeNotFoundError indicates the requested recipe does not exist.
type recipeNotFoundError struct{}

func (e *recipeNotFoundError) Error() string {
	return "recipe not found"
}

// recipeConflictError indicates a valid request that conflicts with the
// current state of the recipe.
type recipeConflictError struct {
	Message string
}

func (e *recipeConflictError) Error() string {
	if e.Message == "" {
		return "recipe conflict"
	}
	return e.Message
}

func mapRecipeUsecaseError(err error) error {
	var v *recipeValidationError
	if errors.As(err, &v) {
		return errValidation(v.FieldErrors)
	}

	var nf *recipeNotFoundError
	if errors.As(err, &nf) {
		return errNotFound()
	}

	var cf *recipeConflictError
	if errors.As(err, &cf) {
		return errConflict(cf.Message)
	}

	return errInternal(err)
}

// resolveIngredientItemID validates item references and creates items when needed.
func resolveIngredientItemID(ctx context.Context, q recipeWorkflowQueries, actorID pgtype.UUID, ingredient recipeIngredientRequest, index int) (pgtype.UUID, error) {
	itemID := ""
	if ingredient.ItemID != nil {
		itemID = strings.TrimSpace(*ingredient.ItemID)
	}
	if itemID != "" {
		parsed, err := uuid.Parse(itemID)
		if err != nil {
			return pgtype.UUID{}, recipeValidationField(fmt.Sprintf("ingredients[%d].item_id", index), "invalid item_id")
		}
		pgID := pgtype.UUID{Bytes: parsed, Valid: true}
		if _, err := q.GetItemByID(ctx, pgID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return pgtype.UUID{}, recipeValidationField(fmt.Sprintf("ingredients[%d].item_id", index), "item does not exist")
			}
			return pgtype.UUID{}, err
		}
		return pgID, nil
	}

	itemName := ""
	if ingredient.ItemName != nil {
		itemName = strings.TrimSpace(*ingredient.ItemName)
	}
	if itemName == "" {
		return pgtype.UUID{}, recipeValidationField(fmt.Sprintf("ingredients[%d].item_name", index), "item_name is required")
	}

	row, err := q.GetItemByName(ctx, itemName)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, err
		}
		created, createErr := q.CreateItem(ctx, sqlc.CreateItemParams{
			Name:      itemName,
			StoreUrl:  pgtype.Text{},
			AisleID:   pgtype.UUID{},
			CreatedBy: actorID,
			UpdatedBy: actorID,
		})
		if createErr != nil {
			if isPGUniqueViolation(createErr) {
				existing, lookupErr := q.GetItemByName(ctx, itemName)
				if lookupErr != nil {
					return pgtype.UUID{}, lookupErr
				}
				return existing.ID, nil
			}
			return pgtype.UUID{}, createErr
		}
		return created.ID, nil
	}
	return row.ID, nil
}

// createRecipeUsecase performs the create-recipe transactional workflow.
func createRecipeUsecase(ctx context.Context, workflows recipeWorkflows, actorID pgtype.UUID, req createRecipeRequest) (pgtype.UUID, error) {
	recipeBookID, err := uuidPtrToPG(req.RecipeBookID)
	if err != nil {
		return pgtype.UUID{}, recipeValidationField("recipe_book_id", "invalid id")
	}

	tagUUIDs, err := uuidsToPG(req.TagIDs)
	if err != nil {
		return pgtype.UUID{}, recipeValidationField("tag_ids", "invalid id")
	}

	if len(tagUUIDs) > 0 {
		count, countErr := workflows.CountTagsByIDs(ctx, tagUUIDs)
		if countErr != nil {
			return pgtype.UUID{}, countErr
		}
		if int(count) != len(tagUUIDs) {
			return pgtype.UUID{}, recipeValidationField("tag_ids", "one or more tags do not exist")
		}
	}

	var recipeID pgtype.UUID
	err = workflows.WithinTx(ctx, func(q recipeWorkflowQueries) error {
		servings32, ok := intToInt32Checked(req.Servings)
		if !ok {
			return recipeValidationField("servings", "servings is too large")
		}
		prepTimeMinutes32, ok := intToInt32Checked(req.PrepTimeMinutes)
		if !ok {
			return recipeValidationField("prep_time_minutes", "prep_time_minutes is too large")
		}
		totalTimeMinutes32, ok := intToInt32Checked(req.TotalTimeMinutes)
		if !ok {
			return recipeValidationField("total_time_minutes", "total_time_minutes is too large")
		}

		row, createRecipeErr := q.CreateRecipe(ctx, sqlc.CreateRecipeParams{
			Title:            strings.TrimSpace(req.Title),
			Servings:         servings32,
			PrepTimeMinutes:  prepTimeMinutes32,
			TotalTimeMinutes: totalTimeMinutes32,
			SourceUrl:        textPtrToPG(req.SourceURL),
			Notes:            textPtrToPG(req.Notes),
			RecipeBookID:     recipeBookID,
			CreatedBy:        actorID,
			UpdatedBy:        actorID,
		})
		if createRecipeErr != nil {
			var pgErr *pgconn.PgError
			if errors.As(createRecipeErr, &pgErr) && pgErr.Code == "23503" {
				return recipeValidationField("recipe_book_id", "recipe book does not exist")
			}
			return createRecipeErr
		}
		recipeID = row.ID

		for i, ing := range req.Ingredients {
			position32, ok := intToInt32Checked(ing.Position)
			if !ok {
				return recipeValidationField("ingredients.position", "position is too large")
			}
			quantity, quantityErr := numericPtrFromFloat64(ing.Quantity)
			if quantityErr != nil {
				return recipeValidationField("ingredients.quantity", "invalid quantity")
			}
			itemID, itemErr := resolveIngredientItemID(ctx, q, actorID, ing, i)
			if itemErr != nil {
				return itemErr
			}
			if createIngredientErr := q.CreateRecipeIngredient(ctx, sqlc.CreateRecipeIngredientParams{
				RecipeID:     recipeID,
				Position:     position32,
				Quantity:     quantity,
				QuantityText: textPtrToPG(ing.QuantityText),
				Unit:         textPtrToPG(ing.Unit),
				ItemID:       itemID,
				Prep:         textPtrToPG(ing.Prep),
				Notes:        textPtrToPG(ing.Notes),
				OriginalText: textPtrToPG(ing.OriginalText),
				CreatedBy:    actorID,
				UpdatedBy:    actorID,
			}); createIngredientErr != nil {
				return createIngredientErr
			}
		}

		for _, step := range req.Steps {
			stepNumber32, ok := intToInt32Checked(step.StepNumber)
			if !ok {
				return recipeValidationField("steps.step_number", "step_number is too large")
			}
			if createStepErr := q.CreateRecipeStep(ctx, sqlc.CreateRecipeStepParams{
				RecipeID:    recipeID,
				StepNumber:  stepNumber32,
				Instruction: strings.TrimSpace(step.Instruction),
				CreatedBy:   actorID,
				UpdatedBy:   actorID,
			}); createStepErr != nil {
				return createStepErr
			}
		}

		for _, tagID := range tagUUIDs {
			if createTagErr := q.CreateRecipeTag(ctx, sqlc.CreateRecipeTagParams{
				RecipeID:  recipeID,
				TagID:     tagID,
				CreatedBy: actorID,
				UpdatedBy: actorID,
			}); createTagErr != nil {
				return createTagErr
			}
		}

		return nil
	})
	if err != nil {
		return pgtype.UUID{}, err
	}

	return recipeID, nil
}

// updateRecipeUsecase performs the update-recipe transactional workflow.
func updateRecipeUsecase(ctx context.Context, workflows recipeWorkflows, actorID pgtype.UUID, recipeID pgtype.UUID, req createRecipeRequest) error {
	recipeBookID, err := uuidPtrToPG(req.RecipeBookID)
	if err != nil {
		return recipeValidationField("recipe_book_id", "invalid id")
	}

	tagUUIDs, err := uuidsToPG(req.TagIDs)
	if err != nil {
		return recipeValidationField("tag_ids", "invalid id")
	}

	if len(tagUUIDs) > 0 {
		count, countErr := workflows.CountTagsByIDs(ctx, tagUUIDs)
		if countErr != nil {
			return countErr
		}
		if int(count) != len(tagUUIDs) {
			return recipeValidationField("tag_ids", "one or more tags do not exist")
		}
	}

	return workflows.WithinTx(ctx, func(q recipeWorkflowQueries) error {
		deletedAt, err := q.GetRecipeDeletedAtByID(ctx, recipeID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return &recipeNotFoundError{}
			}
			return err
		}
		if deletedAt.Valid {
			return &recipeConflictError{Message: "recipe is deleted; restore before updating"}
		}

		servings32, ok := intToInt32Checked(req.Servings)
		if !ok {
			return recipeValidationField("servings", "servings is too large")
		}
		prepTimeMinutes32, ok := intToInt32Checked(req.PrepTimeMinutes)
		if !ok {
			return recipeValidationField("prep_time_minutes", "prep_time_minutes is too large")
		}
		totalTimeMinutes32, ok := intToInt32Checked(req.TotalTimeMinutes)
		if !ok {
			return recipeValidationField("total_time_minutes", "total_time_minutes is too large")
		}

		if _, updateRecipeErr := q.UpdateRecipeByID(ctx, sqlc.UpdateRecipeByIDParams{
			ID:               recipeID,
			Title:            strings.TrimSpace(req.Title),
			Servings:         servings32,
			PrepTimeMinutes:  prepTimeMinutes32,
			TotalTimeMinutes: totalTimeMinutes32,
			SourceUrl:        textPtrToPG(req.SourceURL),
			Notes:            textPtrToPG(req.Notes),
			RecipeBookID:     recipeBookID,
			UpdatedBy:        actorID,
		}); updateRecipeErr != nil {
			var pgErr *pgconn.PgError
			if errors.As(updateRecipeErr, &pgErr) && pgErr.Code == "23503" {
				return recipeValidationField("recipe_book_id", "recipe book does not exist")
			}
			return updateRecipeErr
		}

		if err := q.DeleteRecipeIngredientsByRecipeID(ctx, recipeID); err != nil {
			return err
		}
		if err := q.DeleteRecipeStepsByRecipeID(ctx, recipeID); err != nil {
			return err
		}
		if err := q.DeleteRecipeTagsByRecipeID(ctx, recipeID); err != nil {
			return err
		}

		for i, ing := range req.Ingredients {
			position32, ok := intToInt32Checked(ing.Position)
			if !ok {
				return recipeValidationField("ingredients.position", "position is too large")
			}
			quantity, quantityErr := numericPtrFromFloat64(ing.Quantity)
			if quantityErr != nil {
				return recipeValidationField("ingredients.quantity", "invalid quantity")
			}
			itemID, itemErr := resolveIngredientItemID(ctx, q, actorID, ing, i)
			if itemErr != nil {
				return itemErr
			}
			if createIngredientErr := q.CreateRecipeIngredient(ctx, sqlc.CreateRecipeIngredientParams{
				RecipeID:     recipeID,
				Position:     position32,
				Quantity:     quantity,
				QuantityText: textPtrToPG(ing.QuantityText),
				Unit:         textPtrToPG(ing.Unit),
				ItemID:       itemID,
				Prep:         textPtrToPG(ing.Prep),
				Notes:        textPtrToPG(ing.Notes),
				OriginalText: textPtrToPG(ing.OriginalText),
				CreatedBy:    actorID,
				UpdatedBy:    actorID,
			}); createIngredientErr != nil {
				return createIngredientErr
			}
		}

		for _, step := range req.Steps {
			stepNumber32, ok := intToInt32Checked(step.StepNumber)
			if !ok {
				return recipeValidationField("steps.step_number", "step_number is too large")
			}
			if createStepErr := q.CreateRecipeStep(ctx, sqlc.CreateRecipeStepParams{
				RecipeID:    recipeID,
				StepNumber:  stepNumber32,
				Instruction: strings.TrimSpace(step.Instruction),
				CreatedBy:   actorID,
				UpdatedBy:   actorID,
			}); createStepErr != nil {
				return createStepErr
			}
		}

		for _, tagID := range tagUUIDs {
			if createTagErr := q.CreateRecipeTag(ctx, sqlc.CreateRecipeTagParams{
				RecipeID:  recipeID,
				TagID:     tagID,
				CreatedBy: actorID,
				UpdatedBy: actorID,
			}); createTagErr != nil {
				return createTagErr
			}
		}

		return nil
	})
}

type appRecipeWorkflows struct {
	app *App
}

func (w appRecipeWorkflows) CountTagsByIDs(ctx context.Context, ids []pgtype.UUID) (int32, error) {
	return w.app.queries.CountTagsByIDs(ctx, ids)
}

func (w appRecipeWorkflows) WithinTx(ctx context.Context, fn func(q recipeWorkflowQueries) error) error {
	tx, err := w.app.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			w.app.logger.Warn("rollback failed", "err", rollbackErr)
		}
	}()

	q := w.app.queries.WithTx(tx)
	if err := fn(q); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (a *App) recipeWorkflows() recipeWorkflows {
	return appRecipeWorkflows{app: a}
}
