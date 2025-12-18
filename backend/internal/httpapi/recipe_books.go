package httpapi

import (
	"errors"
	"net/http"
	"strings"

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

func (a *App) handleRecipeBooksList(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	books, err := a.queries.ListRecipeBooks(r.Context())
	if err != nil {
		return errInternal(err)
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
	return nil
}

func (a *App) handleRecipeBooksCreate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	var req createRecipeBookRequest
	if err := a.decodeJSON(w, r, &req); err != nil {
		return err
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return errValidationField("name", "name is required")
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	row, err := a.queries.CreateRecipeBook(r.Context(), sqlc.CreateRecipeBookParams{
		Name:      req.Name,
		CreatedBy: userID,
		UpdatedBy: userID,
	})
	if err != nil {
		if isPGUniqueViolation(err) {
			return errValidationField("name", "name already exists")
		}
		return errInternal(err)
	}

	resp := recipeBookResponse{
		ID:        uuidString(row.ID),
		Name:      row.Name,
		CreatedAt: timeString(row.CreatedAt),
	}

	if err := response.WriteJSON(w, http.StatusOK, resp); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/recipe-books")
	}
	return nil
}

func (a *App) handleRecipeBooksUpdate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	var req updateRecipeBookRequest
	if decodeErr := a.decodeJSON(w, r, &req); decodeErr != nil {
		return decodeErr
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return errValidationField("name", "name is required")
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	row, err := a.queries.UpdateRecipeBookByID(r.Context(), sqlc.UpdateRecipeBookByIDParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		Name:      req.Name,
		UpdatedBy: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		if isPGUniqueViolation(err) {
			return errValidationField("name", "name already exists")
		}
		return errInternal(err)
	}

	resp := recipeBookResponse{
		ID:        uuidString(row.ID),
		Name:      row.Name,
		CreatedAt: timeString(row.CreatedAt),
	}

	if err := response.WriteJSON(w, http.StatusOK, resp); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/recipe-books")
	}
	return nil
}

func (a *App) handleRecipeBooksDelete(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	affected, err := a.queries.DeleteRecipeBookByID(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return errConflict("cannot delete recipe book with recipes")
		}
		return errInternal(err)
	}
	if affected == 0 {
		return errNotFound()
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
