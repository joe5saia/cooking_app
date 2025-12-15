package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

func (a *App) handleRecipesDelete(w http.ResponseWriter, r *http.Request) {
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

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	affected, err := a.queries.SoftDeleteRecipeByID(r.Context(), sqlc.SoftDeleteRecipeByIDParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		UpdatedBy: userID,
	})
	if err != nil {
		a.logger.Error("soft delete recipe failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}
	if affected == 0 {
		a.writeProblem(w, http.StatusNotFound, "not_found", "not found", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleRecipesRestore(w http.ResponseWriter, r *http.Request) {
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

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	affected, err := a.queries.RestoreRecipeByID(r.Context(), sqlc.RestoreRecipeByIDParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		UpdatedBy: userID,
	})
	if err != nil {
		a.logger.Error("restore recipe failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}
	if affected == 0 {
		a.writeProblem(w, http.StatusNotFound, "not_found", "not found", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
