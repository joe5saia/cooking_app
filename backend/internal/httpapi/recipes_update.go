package httpapi

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

func (a *App) handleRecipesUpdate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}
	recipeID := pgtype.UUID{Bytes: id, Valid: true}

	var req createRecipeRequest
	if decodeErr := a.decodeJSON(w, r, &req); decodeErr != nil {
		return decodeErr
	}

	if errs := validateCreateRecipeRequest(req); len(errs) > 0 {
		return errValidation(errs)
	}

	ctx := r.Context()
	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}

	if updateErr := updateRecipeUsecase(ctx, a.recipeWorkflows(), userID, recipeID, req); updateErr != nil {
		return mapRecipeUsecaseError(updateErr)
	}

	detail, err := a.loadRecipeDetail(ctx, recipeID)
	if err != nil {
		return errInternal(err)
	}

	if err := response.WriteJSON(w, http.StatusOK, detail); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/recipes/{id}")
	}
	return nil
}
