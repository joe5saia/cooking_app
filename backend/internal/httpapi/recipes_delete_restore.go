package httpapi

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
)

func (a *App) handleRecipesDelete(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	affected, err := a.queries.SoftDeleteRecipeByID(r.Context(), sqlc.SoftDeleteRecipeByIDParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		UpdatedBy: userID,
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

func (a *App) handleRecipesRestore(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	affected, err := a.queries.RestoreRecipeByID(r.Context(), sqlc.RestoreRecipeByIDParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		UpdatedBy: userID,
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
