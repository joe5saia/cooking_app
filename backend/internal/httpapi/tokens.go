package httpapi

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/auth/pat"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

type createTokenRequest struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}

func (a *App) handleTokensList(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	tokens, err := a.queries.ListTokensByUser(r.Context(), userID)
	if err != nil {
		return errInternal(err)
	}

	if err := response.WriteJSON(w, http.StatusOK, patListResponse(tokens)); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/tokens")
	}
	return nil
}

func (a *App) handleTokensCreate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}
	if a.tokenCreateLimiter != nil && !a.tokenCreateLimiter.allow(info.UserID.String()) {
		a.audit(r, "token.create.rate_limited")
		return errRateLimited()
	}
	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}

	var req createTokenRequest
	if err := a.decodeJSON(w, r, &req); err != nil {
		return err
	}
	if req.Name == "" {
		return errValidationField("name", "name is required")
	}

	var expiresAt pgtype.Timestamptz
	if req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			return errValidationField("expires_at", "expires_at must be RFC3339")
		}
		expiresAt = pgtype.Timestamptz{Time: parsed, Valid: true}
	}

	secret, hash, err := pat.Generate()
	if err != nil {
		a.audit(r, "token.create.error")
		return errInternal(err)
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
		a.audit(r, "token.create.error")
		return errInternal(err)
	}

	a.audit(r, "token.created", "token_id", uuidString(row.ID), "name", row.Name, "expires_at", timeString(row.ExpiresAt))

	resp := map[string]any{
		"id":         uuidString(row.ID),
		"name":       row.Name,
		"token":      secret,
		"created_at": row.CreatedAt.Time.UTC().Format(time.RFC3339Nano),
	}

	if err := response.WriteJSON(w, http.StatusOK, resp); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/tokens")
	}
	return nil
}

func (a *App) handleTokensDelete(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}
	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	affected, err := a.queries.DeleteTokenByIDForUser(r.Context(), sqlc.DeleteTokenByIDForUserParams{
		ID:     pgtype.UUID{Bytes: id, Valid: true},
		UserID: userID,
	})
	if err != nil {
		a.audit(r, "token.delete.error", "token_id", id.String())
		return errInternal(err)
	}
	if affected == 0 {
		a.audit(r, "token.delete.not_found", "token_id", id.String())
		return errNotFound()
	}

	a.audit(r, "token.deleted", "token_id", id.String())
	w.WriteHeader(http.StatusNoContent)
	return nil
}
