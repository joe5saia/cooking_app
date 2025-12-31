package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

type recipeTagResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type recipeIngredientResponse struct {
	ID           string       `json:"id"`
	Position     int          `json:"position"`
	Quantity     *float64     `json:"quantity"`
	QuantityText *string      `json:"quantity_text"`
	Unit         *string      `json:"unit"`
	Item         itemResponse `json:"item"`
	Prep         *string      `json:"prep"`
	Notes        *string      `json:"notes"`
	OriginalText *string      `json:"original_text"`
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

func (a *App) handleRecipesCreate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	var req createRecipeRequest
	if err := a.decodeJSON(w, r, &req); err != nil {
		return err
	}

	if errs := validateCreateRecipeRequest(req); len(errs) > 0 {
		return errValidation(errs)
	}

	ctx := r.Context()
	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}

	recipeID, err := createRecipeUsecase(ctx, a.recipeWorkflows(), userID, req)
	if err != nil {
		return mapRecipeUsecaseError(err)
	}

	detail, err := a.loadRecipeDetail(ctx, recipeID)
	if err != nil {
		return errInternal(err)
	}

	if err := response.WriteJSON(w, http.StatusCreated, detail); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/recipes")
	}
	return nil
}

func (a *App) handleRecipesGet(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	detail, err := a.loadRecipeDetail(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		return errInternal(err)
	}

	if err := response.WriteJSON(w, http.StatusOK, detail); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/recipes/{id}")
	}
	return nil
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
		var quantity *float64
		if ing.Quantity.Valid {
			f8, err := ing.Quantity.Float64Value()
			if err != nil {
				return recipeDetailResponse{}, err
			}
			if f8.Valid {
				q := f8.Float64
				quantity = &q
			}
		}
		outIngredients = append(outIngredients, recipeIngredientResponse{
			ID:           uuidString(ing.ID),
			Position:     int(ing.Position),
			Quantity:     quantity,
			QuantityText: textStringPtr(ing.QuantityText),
			Unit:         textStringPtr(ing.Unit),
			Item: itemResponse{
				ID:       uuidString(ing.ItemID),
				Name:     ing.ItemName,
				StoreURL: textStringPtr(ing.ItemStoreUrl),
				Aisle: buildAisleResponse(
					ing.ItemAisleID,
					ing.AisleName,
					ing.AisleSortGroup,
					ing.AisleSortOrder,
					ing.AisleNumericValue,
				),
			},
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
