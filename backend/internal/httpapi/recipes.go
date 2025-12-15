package httpapi

import (
	"context"
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

type recipeTagResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type recipeIngredientResponse struct {
	ID           string          `json:"id"`
	Position     int             `json:"position"`
	Quantity     *pgtype.Numeric `json:"quantity"`
	QuantityText *string         `json:"quantity_text"`
	Unit         *string         `json:"unit"`
	Item         string          `json:"item"`
	Prep         *string         `json:"prep"`
	Notes        *string         `json:"notes"`
	OriginalText *string         `json:"original_text"`
}

type recipeStepResponse struct {
	ID          string `json:"id"`
	StepNumber  int    `json:"step_number"`
	Instruction string `json:"instruction"`
}

type recipeDetailResponse struct {
	ID               string                     `json:"id"`
	Title            string                     `json:"title"`
	Servings         int32                      `json:"servings"`
	PrepTimeMinutes  int32                      `json:"prep_time_minutes"`
	TotalTimeMinutes int32                      `json:"total_time_minutes"`
	SourceURL        *string                    `json:"source_url"`
	Notes            *string                    `json:"notes"`
	RecipeBookID     *string                    `json:"recipe_book_id"`
	Tags             []recipeTagResponse        `json:"tags"`
	Ingredients      []recipeIngredientResponse `json:"ingredients"`
	Steps            []recipeStepResponse       `json:"steps"`
	CreatedAt        string                     `json:"created_at"`
	CreatedBy        string                     `json:"created_by"`
	UpdatedAt        string                     `json:"updated_at"`
	UpdatedBy        string                     `json:"updated_by"`
	DeletedAt        *string                    `json:"deleted_at"`
}

func (a *App) handleRecipesCreate(w http.ResponseWriter, r *http.Request) {
	info, ok := authInfoFromRequest(r)
	if !ok {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	var req createRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	q := a.queries.WithTx(tx)
	recipeRow, err := q.CreateRecipe(ctx, sqlc.CreateRecipeParams{
		Title:            strings.TrimSpace(req.Title),
		Servings:         servings32,
		PrepTimeMinutes:  prepTimeMinutes32,
		TotalTimeMinutes: totalTimeMinutes32,
		SourceUrl:        textPtrToPG(req.SourceURL),
		Notes:            textPtrToPG(req.Notes),
		RecipeBookID:     recipeBookID,
		CreatedBy:        userID,
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
		a.logger.Error("create recipe failed", "err", err)
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
			RecipeID:     recipeRow.ID,
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
			RecipeID:    recipeRow.ID,
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
			RecipeID:  recipeRow.ID,
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

	detail, err := a.loadRecipeDetail(ctx, recipeRow.ID)
	if err != nil {
		a.logger.Error("load recipe detail failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	if err := response.WriteJSON(w, http.StatusCreated, detail); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/recipes")
	}
}

func (a *App) handleRecipesGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := authInfoFromRequest(r); !ok {
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

	detail, err := a.loadRecipeDetail(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.writeProblem(w, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		a.logger.Error("load recipe detail failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, detail); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/recipes/{id}")
	}
}

func (a *App) loadRecipeDetail(ctx context.Context, id pgtype.UUID) (recipeDetailResponse, error) {
	row, err := a.queries.GetRecipeByID(ctx, id)
	if err != nil {
		return recipeDetailResponse{}, err
	}

	ingredients, err := a.queries.ListRecipeIngredientsByRecipeID(ctx, id)
	if err != nil {
		return recipeDetailResponse{}, err
	}
	steps, err := a.queries.ListRecipeStepsByRecipeID(ctx, id)
	if err != nil {
		return recipeDetailResponse{}, err
	}
	tags, err := a.queries.ListRecipeTagsByRecipeID(ctx, id)
	if err != nil {
		return recipeDetailResponse{}, err
	}

	outIngredients := make([]recipeIngredientResponse, 0, len(ingredients))
	for _, ing := range ingredients {
		var quantity *pgtype.Numeric
		if ing.Quantity.Valid {
			q := ing.Quantity
			quantity = &q
		}
		outIngredients = append(outIngredients, recipeIngredientResponse{
			ID:           uuidString(ing.ID),
			Position:     int(ing.Position),
			Quantity:     quantity,
			QuantityText: textStringPtr(ing.QuantityText),
			Unit:         textStringPtr(ing.Unit),
			Item:         ing.Item,
			Prep:         textStringPtr(ing.Prep),
			Notes:        textStringPtr(ing.Notes),
			OriginalText: textStringPtr(ing.OriginalText),
		})
	}

	outSteps := make([]recipeStepResponse, 0, len(steps))
	for _, s := range steps {
		outSteps = append(outSteps, recipeStepResponse{
			ID:          uuidString(s.ID),
			StepNumber:  int(s.StepNumber),
			Instruction: s.Instruction,
		})
	}

	outTags := make([]recipeTagResponse, 0, len(tags))
	for _, t := range tags {
		outTags = append(outTags, recipeTagResponse{ID: uuidString(t.ID), Name: t.Name})
	}

	return recipeDetailResponse{
		ID:               uuidString(row.ID),
		Title:            row.Title,
		Servings:         row.Servings,
		PrepTimeMinutes:  row.PrepTimeMinutes,
		TotalTimeMinutes: row.TotalTimeMinutes,
		SourceURL:        textStringPtr(row.SourceUrl),
		Notes:            textStringPtr(row.Notes),
		RecipeBookID:     uuidStringPtr(row.RecipeBookID),
		Tags:             outTags,
		Ingredients:      outIngredients,
		Steps:            outSteps,
		CreatedAt:        timeString(row.CreatedAt),
		CreatedBy:        uuidString(row.CreatedBy),
		UpdatedAt:        timeString(row.UpdatedAt),
		UpdatedBy:        uuidString(row.UpdatedBy),
		DeletedAt:        timeStringPtr(row.DeletedAt),
	}, nil
}

func textPtrToPG(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: trimmed, Valid: true}
}

func textStringPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func uuidStringPtr(v pgtype.UUID) *string {
	if !v.Valid {
		return nil
	}
	s := uuid.UUID(v.Bytes).String()
	return &s
}

func numericPtrFromFloat64(v *float64) (pgtype.Numeric, error) {
	if v == nil {
		return pgtype.Numeric{}, nil
	}
	var n pgtype.Numeric
	if err := n.Scan(*v); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

func uuidPtrToPG(id *string) (pgtype.UUID, error) {
	if id == nil {
		return pgtype.UUID{}, nil
	}
	trimmed := strings.TrimSpace(*id)
	if trimmed == "" {
		return pgtype.UUID{}, nil
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func uuidsToPG(ids []string) ([]pgtype.UUID, error) {
	out := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		parsed, err := uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		out = append(out, pgtype.UUID{Bytes: parsed, Valid: true})
	}
	return out, nil
}

func timeStringPtr(v pgtype.Timestamptz) *string {
	if !v.Valid {
		return nil
	}
	s := timeString(v)
	return &s
}

func intToInt32Checked(v int) (int32, bool) {
	if v > maxInt32 {
		return 0, false
	}
	minInt32 := -maxInt32 - 1
	if v < minInt32 {
		return 0, false
	}
	return int32(v), true //nolint:gosec // bounds checked above
}
