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
		info, err := a.authenticateRequest(r)
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		if info.AuthType == authTypeSession && isUnsafeMethod(r.Method) && !a.isCSRFValid(r) {
			a.writeError(w, r, errForbidden("csrf token missing or invalid"))
			return
		}
		*r = *r.WithContext(withAuthInfo(r.Context(), info))
		next.ServeHTTP(w, r)
	})
}

func (a *App) authenticateRequest(r *http.Request) (authInfo, error) {
	if token, ok := parseBearerToken(r.Header.Get("Authorization")); ok {
		return a.authenticatePAT(r, token)
	}

	if a.readSessionCookie(r) != "" {
		return a.authenticateSession(r)
	}

	return authInfo{}, errUnauthorized("unauthorized")
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

func (a *App) authenticateSession(r *http.Request) (authInfo, error) {
	u, err := a.requireSessionUser(r)
	if err != nil {
		return authInfo{}, err
	}
	if u == nil || !u.UserID.Valid {
		return authInfo{}, errUnauthorized("unauthorized")
	}

	return authInfo{
		UserID:   uuid.UUID(u.UserID.Bytes),
		AuthType: authTypeSession,
	}, nil
}

func (a *App) authenticatePAT(r *http.Request, token string) (authInfo, error) {
	if err := pat.ValidateSecret(token); err != nil {
		return authInfo{}, errUnauthorized("unauthorized")
	}

	hash := pat.Hash(token)
	row, err := a.queries.GetTokenUserByHash(r.Context(), hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authInfo{}, errUnauthorized("unauthorized")
		}
		return authInfo{}, errInternal(err)
	}

	if row.TokenExpiresAt.Valid && time.Now().After(row.TokenExpiresAt.Time) {
		return authInfo{}, errUnauthorized("unauthorized")
	}

	if !row.IsActive || !row.UserID.Valid || !row.TokenID.Valid {
		return authInfo{}, errUnauthorized("unauthorized")
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
	}, nil
}
