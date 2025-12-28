package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

const mealPlanDateLayout = "2006-01-02"

type mealPlanRecipeResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type mealPlanEntryResponse struct {
	Date   string                 `json:"date"`
	Recipe mealPlanRecipeResponse `json:"recipe"`
}

type mealPlanListResponse struct {
	Items []mealPlanEntryResponse `json:"items"`
}

type createMealPlanRequest struct {
	Date     string `json:"date"`
	RecipeID string `json:"recipe_id"`
}

// parseMealPlanDate parses a YYYY-MM-DD date string into a PG date value.
func parseMealPlanDate(field, value string) (pgtype.Date, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return pgtype.Date{}, errValidationField(field, "required")
	}
	parsed, err := time.Parse(mealPlanDateLayout, trimmed)
	if err != nil {
		return pgtype.Date{}, errValidationField(field, "invalid date")
	}
	return pgtype.Date{Time: parsed.UTC(), Valid: true}, nil
}

// mealPlanDateString formats a PG date value for API responses.
func mealPlanDateString(v pgtype.Date) string {
	if !v.Valid {
		return ""
	}
	return v.Time.UTC().Format(mealPlanDateLayout)
}

// handleMealPlansList lists meal plan entries for the authenticated user.
func (a *App) handleMealPlansList(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	qp := r.URL.Query()
	start, err := parseMealPlanDate("start", qp.Get("start"))
	if err != nil {
		return err
	}
	end, err := parseMealPlanDate("end", qp.Get("end"))
	if err != nil {
		return err
	}
	if start.Time.After(end.Time) {
		return errValidationField("end", "end must be on or after start")
	}

	rows, err := a.queries.ListMealPlanEntriesByRange(r.Context(), sqlc.ListMealPlanEntriesByRangeParams{
		UserID:    pgtype.UUID{Bytes: info.UserID, Valid: true},
		StartDate: start,
		EndDate:   end,
	})
	if err != nil {
		return errInternal(err)
	}

	items := make([]mealPlanEntryResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mealPlanEntryResponse{
			Date: mealPlanDateString(row.PlanDate),
			Recipe: mealPlanRecipeResponse{
				ID:    uuidString(row.RecipeID),
				Title: row.Title,
			},
		})
	}

	if err := response.WriteJSON(w, http.StatusOK, mealPlanListResponse{Items: items}); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/meal-plans")
	}
	return nil
}

// handleMealPlansCreate creates a new meal plan entry for the authenticated user.
func (a *App) handleMealPlansCreate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	var req createMealPlanRequest
	if err := a.decodeJSON(w, r, &req); err != nil {
		return err
	}

	planDate, err := parseMealPlanDate("date", req.Date)
	if err != nil {
		return err
	}

	recipeIDStr := strings.TrimSpace(req.RecipeID)
	if recipeIDStr == "" {
		return errValidationField("recipe_id", "required")
	}
	recipeID, err := uuid.Parse(recipeIDStr)
	if err != nil {
		return errValidationField("recipe_id", "invalid id")
	}

	row, err := a.queries.CreateMealPlanEntry(r.Context(), sqlc.CreateMealPlanEntryParams{
		RecipeID:  pgtype.UUID{Bytes: recipeID, Valid: true},
		UserID:    pgtype.UUID{Bytes: info.UserID, Valid: true},
		PlanDate:  planDate,
		CreatedBy: pgtype.UUID{Bytes: info.UserID, Valid: true},
		UpdatedBy: pgtype.UUID{Bytes: info.UserID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errValidationField("recipe_id", "recipe does not exist")
		}
		if isPGUniqueViolation(err) {
			return errConflict("meal plan entry already exists")
		}
		return errInternal(err)
	}

	resp := mealPlanEntryResponse{
		Date: mealPlanDateString(row.PlanDate),
		Recipe: mealPlanRecipeResponse{
			ID:    uuidString(row.RecipeID),
			Title: row.Title,
		},
	}
	if err := response.WriteJSON(w, http.StatusOK, resp); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/meal-plans")
	}
	return nil
}

// handleMealPlansDelete deletes a meal plan entry for the authenticated user.
func (a *App) handleMealPlansDelete(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	planDate, err := parseMealPlanDate("date", chi.URLParam(r, "date"))
	if err != nil {
		return err
	}

	recipeID, err := parseUUIDParam(r, "recipe_id")
	if err != nil {
		return err
	}

	affected, err := a.queries.DeleteMealPlanEntry(r.Context(), sqlc.DeleteMealPlanEntryParams{
		UserID:   pgtype.UUID{Bytes: info.UserID, Valid: true},
		PlanDate: planDate,
		RecipeID: pgtype.UUID{Bytes: recipeID, Valid: true},
	})
	if err != nil {
		return errInternal(err)
	}
	if affected == 0 {
		return errNotFound()
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
