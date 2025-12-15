package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

func (a *App) handleRecipesUpdate(w http.ResponseWriter, r *http.Request) {
	info, ok := authInfoFromRequest(r)
	if !ok {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		a.writeProblem(w, http.StatusBadRequest, "validation_error", "invalid id", []response.FieldError{
			{Field: "id", Message: "invalid id"},
		})
		return
	}
	recipeID := pgtype.UUID{Bytes: id, Valid: true}

	var req createRecipeRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		a.writeProblem(w, http.StatusBadRequest, "bad_request", "invalid JSON", nil)
		return
	}

	if errs := validateCreateRecipeRequest(req); len(errs) > 0 {
		a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", errs)
		return
	}

	recipeBookID, err := uuidPtrToPG(req.RecipeBookID)
	if err != nil {
		a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
			{Field: "recipe_book_id", Message: "invalid id"},
		})
		return
	}

	tagUUIDs, err := uuidsToPG(req.TagIDs)
	if err != nil {
		a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
			{Field: "tag_ids", Message: "invalid id"},
		})
		return
	}

	ctx := r.Context()
	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}

	if len(tagUUIDs) > 0 {
		count, countErr := a.queries.CountTagsByIDs(ctx, tagUUIDs)
		if countErr != nil {
			a.logger.Error("count tags failed", "err", countErr)
			a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
			return
		}
		if int(count) != len(tagUUIDs) {
			a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
				{Field: "tag_ids", Message: "one or more tags do not exist"},
			})
			return
		}
	}

	tx, err := a.pool.Begin(ctx)
	if err != nil {
		a.logger.Error("begin tx failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			a.logger.Warn("rollback failed", "err", rollbackErr)
		}
	}()

	q := a.queries.WithTx(tx)
	deletedAt, err := q.GetRecipeDeletedAtByID(ctx, recipeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.writeProblem(w, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		a.logger.Error("get recipe deleted_at failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}
	if deletedAt.Valid {
		a.writeProblem(w, http.StatusConflict, "conflict", "recipe is deleted; restore before updating", nil)
		return
	}

	servings32, ok := intToInt32Checked(req.Servings)
	if !ok {
		a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
			{Field: "servings", Message: "servings is too large"},
		})
		return
	}
	prepTimeMinutes32, ok := intToInt32Checked(req.PrepTimeMinutes)
	if !ok {
		a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
			{Field: "prep_time_minutes", Message: "prep_time_minutes is too large"},
		})
		return
	}
	totalTimeMinutes32, ok := intToInt32Checked(req.TotalTimeMinutes)
	if !ok {
		a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
			{Field: "total_time_minutes", Message: "total_time_minutes is too large"},
		})
		return
	}

	_, err = q.UpdateRecipeByID(ctx, sqlc.UpdateRecipeByIDParams{
		ID:               recipeID,
		Title:            strings.TrimSpace(req.Title),
		Servings:         servings32,
		PrepTimeMinutes:  prepTimeMinutes32,
		TotalTimeMinutes: totalTimeMinutes32,
		SourceUrl:        textPtrToPG(req.SourceURL),
		Notes:            textPtrToPG(req.Notes),
		RecipeBookID:     recipeBookID,
		UpdatedBy:        userID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
				{Field: "recipe_book_id", Message: "recipe book does not exist"},
			})
			return
		}
		a.logger.Error("update recipe failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	if deleteErr := q.DeleteRecipeIngredientsByRecipeID(ctx, recipeID); deleteErr != nil {
		a.logger.Error("delete ingredients failed", "err", deleteErr)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}
	if deleteErr := q.DeleteRecipeStepsByRecipeID(ctx, recipeID); deleteErr != nil {
		a.logger.Error("delete steps failed", "err", deleteErr)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}
	if deleteErr := q.DeleteRecipeTagsByRecipeID(ctx, recipeID); deleteErr != nil {
		a.logger.Error("delete tags failed", "err", deleteErr)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	for _, ing := range req.Ingredients {
		position32, ok := intToInt32Checked(ing.Position)
		if !ok {
			a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
				{Field: "ingredients.position", Message: "position is too large"},
			})
			return
		}
		quantity, quantityErr := numericPtrFromFloat64(ing.Quantity)
		if quantityErr != nil {
			a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
				{Field: "ingredients.quantity", Message: "invalid quantity"},
			})
			return
		}
		if createErr := q.CreateRecipeIngredient(ctx, sqlc.CreateRecipeIngredientParams{
			RecipeID:     recipeID,
			Position:     position32,
			Quantity:     quantity,
			QuantityText: textPtrToPG(ing.QuantityText),
			Unit:         textPtrToPG(ing.Unit),
			Item:         strings.TrimSpace(ing.Item),
			Prep:         textPtrToPG(ing.Prep),
			Notes:        textPtrToPG(ing.Notes),
			OriginalText: textPtrToPG(ing.OriginalText),
			CreatedBy:    userID,
			UpdatedBy:    userID,
		}); createErr != nil {
			a.logger.Error("create ingredient failed", "err", createErr)
			a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
			return
		}
	}

	for _, step := range req.Steps {
		stepNumber32, ok := intToInt32Checked(step.StepNumber)
		if !ok {
			a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
				{Field: "steps.step_number", Message: "step_number is too large"},
			})
			return
		}
		if createErr := q.CreateRecipeStep(ctx, sqlc.CreateRecipeStepParams{
			RecipeID:    recipeID,
			StepNumber:  stepNumber32,
			Instruction: strings.TrimSpace(step.Instruction),
			CreatedBy:   userID,
			UpdatedBy:   userID,
		}); createErr != nil {
			a.logger.Error("create step failed", "err", createErr)
			a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
			return
		}
	}

	for _, tagID := range tagUUIDs {
		if createErr := q.CreateRecipeTag(ctx, sqlc.CreateRecipeTagParams{
			RecipeID:  recipeID,
			TagID:     tagID,
			CreatedBy: userID,
			UpdatedBy: userID,
		}); createErr != nil {
			a.logger.Error("create recipe tag failed", "err", createErr)
			a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
			return
		}
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		a.logger.Error("commit failed", "err", commitErr)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	detail, err := a.loadRecipeDetail(ctx, recipeID)
	if err != nil {
		a.logger.Error("load recipe detail failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, detail); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/recipes/{id}")
	}
}
