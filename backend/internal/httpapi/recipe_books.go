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

type createRecipeBookRequest struct {
	Name string `json:"name"`
}

type updateRecipeBookRequest struct {
	Name string `json:"name"`
}

type recipeBookResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

func (a *App) handleRecipeBooksList(w http.ResponseWriter, r *http.Request) {
	if _, ok := authInfoFromRequest(r); !ok {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	books, err := a.queries.ListRecipeBooks(r.Context())
	if err != nil {
		a.logger.Error("list recipe books failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	out := make([]recipeBookResponse, 0, len(books))
	for _, b := range books {
		out = append(out, recipeBookResponse{
			ID:        uuidString(b.ID),
			Name:      b.Name,
			CreatedAt: timeString(b.CreatedAt),
		})
	}

	if err := response.WriteJSON(w, http.StatusOK, out); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/recipe-books")
	}
}

func (a *App) handleRecipeBooksCreate(w http.ResponseWriter, r *http.Request) {
	info, ok := authInfoFromRequest(r)
	if !ok {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	var req createRecipeBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeProblem(w, http.StatusBadRequest, "bad_request", "invalid JSON", nil)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		a.writeValidation(w, "name", "name is required")
		return
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	row, err := a.queries.CreateRecipeBook(r.Context(), sqlc.CreateRecipeBookParams{
		Name:      req.Name,
		CreatedBy: userID,
		UpdatedBy: userID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			a.writeValidation(w, "name", "name already exists")
			return
		}
		a.logger.Error("create recipe book failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	resp := recipeBookResponse{
		ID:        uuidString(row.ID),
		Name:      row.Name,
		CreatedAt: timeString(row.CreatedAt),
	}

	if err := response.WriteJSON(w, http.StatusOK, resp); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/recipe-books")
	}
}

func (a *App) handleRecipeBooksUpdate(w http.ResponseWriter, r *http.Request) {
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

	var req updateRecipeBookRequest
	decodeErr := json.NewDecoder(r.Body).Decode(&req)
	if decodeErr != nil {
		a.writeProblem(w, http.StatusBadRequest, "bad_request", "invalid JSON", nil)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		a.writeValidation(w, "name", "name is required")
		return
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	row, err := a.queries.UpdateRecipeBookByID(r.Context(), sqlc.UpdateRecipeBookByIDParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		Name:      req.Name,
		UpdatedBy: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.writeProblem(w, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			a.writeValidation(w, "name", "name already exists")
			return
		}
		a.logger.Error("update recipe book failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	resp := recipeBookResponse{
		ID:        uuidString(row.ID),
		Name:      row.Name,
		CreatedAt: timeString(row.CreatedAt),
	}

	if err := response.WriteJSON(w, http.StatusOK, resp); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/recipe-books")
	}
}

func (a *App) handleRecipeBooksDelete(w http.ResponseWriter, r *http.Request) {
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

	affected, err := a.queries.DeleteRecipeBookByID(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			a.writeProblem(w, http.StatusConflict, "conflict", "cannot delete recipe book with recipes", nil)
			return
		}
		a.logger.Error("delete recipe book failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}
	if affected == 0 {
		a.writeProblem(w, http.StatusNotFound, "not_found", "not found", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
