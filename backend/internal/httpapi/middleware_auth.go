package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/saiaj/cooking_app/backend/internal/auth/pat"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
)

func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := a.authenticateRequest(w, r)
		if !ok {
			return
		}
		*r = *r.WithContext(withAuthInfo(r.Context(), info))
		next.ServeHTTP(w, r)
	})
}

func (a *App) authenticateRequest(w http.ResponseWriter, r *http.Request) (authInfo, bool) {
	if token, ok := parseBearerToken(r.Header.Get("Authorization")); ok {
		info, ok := a.authenticatePAT(w, r, token)
		if ok {
			return info, true
		}
		return authInfo{}, false
	}

	if a.readSessionCookie(r) != "" {
		info, ok := a.authenticateSession(w, r)
		if ok {
			return info, true
		}
		return authInfo{}, false
	}

	a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
	return authInfo{}, false
}

func parseBearerToken(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	kind, value, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(kind, "Bearer") {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

func (a *App) authenticateSession(w http.ResponseWriter, r *http.Request) (authInfo, bool) {
	u, ok := a.requireSessionUser(w, r)
	if !ok || u == nil || !u.UserID.Valid {
		return authInfo{}, false
	}

	return authInfo{
		UserID:   uuid.UUID(u.UserID.Bytes),
		AuthType: authTypeSession,
	}, true
}

func (a *App) authenticatePAT(w http.ResponseWriter, r *http.Request, token string) (authInfo, bool) {
	if err := pat.ValidateSecret(token); err != nil {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return authInfo{}, false
	}

	hash := pat.Hash(token)
	row, err := a.queries.GetTokenUserByHash(r.Context(), hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
			return authInfo{}, false
		}
		a.logger.Error("get token failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return authInfo{}, false
	}

	if row.TokenExpiresAt.Valid && time.Now().After(row.TokenExpiresAt.Time) {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return authInfo{}, false
	}

	if !row.IsActive || !row.UserID.Valid || !row.TokenID.Valid {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return authInfo{}, false
	}

	if err := a.queries.TouchTokenLastUsed(r.Context(), sqlc.TouchTokenLastUsedParams{
		ID:        row.TokenID,
		UpdatedBy: row.UserID,
	}); err != nil {
		a.logger.Warn("touch token last_used_at failed", "err", err)
	}

	return authInfo{
		UserID:   uuid.UUID(row.UserID.Bytes),
		AuthType: authTypePAT,
	}, true
}
