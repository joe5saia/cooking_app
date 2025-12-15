package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/auth/pat"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

type createTokenRequest struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}

func (a *App) handleTokensList(w http.ResponseWriter, r *http.Request) {
	info, ok := authInfoFromRequest(r)
	if !ok {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	tokens, err := a.queries.ListTokensByUser(r.Context(), userID)
	if err != nil {
		a.logger.Error("list tokens failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	if err := response.WriteJSON(w, http.StatusOK, patListResponse(tokens)); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/tokens")
	}
}

func (a *App) handleTokensCreate(w http.ResponseWriter, r *http.Request) {
	info, ok := authInfoFromRequest(r)
	if !ok {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeProblem(w, http.StatusBadRequest, "bad_request", "invalid JSON", nil)
		return
	}
	if req.Name == "" {
		a.writeValidation(w, "name", "name is required")
		return
	}

	var expiresAt pgtype.Timestamptz
	if req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			a.writeValidation(w, "expires_at", "expires_at must be RFC3339")
			return
		}
		expiresAt = pgtype.Timestamptz{Time: parsed, Valid: true}
	}

	secret, hash, err := pat.Generate()
	if err != nil {
		a.logger.Error("token generation failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	row, err := a.queries.CreateToken(r.Context(), sqlc.CreateTokenParams{
		UserID:     userID,
		Name:       req.Name,
		TokenHash:  hash,
		LastUsedAt: pgtype.Timestamptz{},
		ExpiresAt:  expiresAt,
		CreatedBy:  userID,
		UpdatedBy:  userID,
	})
	if err != nil {
		a.logger.Error("create token failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	resp := map[string]any{
		"id":         uuid.UUID(row.ID.Bytes).String(),
		"name":       row.Name,
		"token":      secret,
		"created_at": row.CreatedAt.Time.UTC().Format(time.RFC3339Nano),
	}

	if err := response.WriteJSON(w, http.StatusOK, resp); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/tokens")
	}
}

func (a *App) handleTokensDelete(w http.ResponseWriter, r *http.Request) {
	info, ok := authInfoFromRequest(r)
	if !ok {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		a.writeProblem(w, http.StatusBadRequest, "validation_error", "invalid id", []response.FieldError{
			{Field: "id", Message: "invalid id"},
		})
		return
	}

	affected, err := a.queries.DeleteTokenByIDForUser(r.Context(), sqlc.DeleteTokenByIDForUserParams{
		ID:     pgtype.UUID{Bytes: id, Valid: true},
		UserID: userID,
	})
	if err != nil {
		a.logger.Error("delete token failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}
	if affected == 0 {
		a.writeProblem(w, http.StatusNotFound, "not_found", "not found", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
