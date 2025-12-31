package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/auth/password"
	"github.com/saiaj/cooking_app/backend/internal/auth/users"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type meResponse struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName *string `json:"display_name"`
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) error {
	var req loginRequest
	if err := a.decodeJSON(w, r, &req); err != nil {
		return err
	}

	username, err := users.NormalizeUsername(req.Username)
	if err != nil {
		a.audit(r, "auth.login.failed", "reason", "invalid_username")
		return errValidationField("username", err.Error())
	}
	if strings.TrimSpace(req.Password) == "" {
		a.audit(r, "auth.login.failed", "reason", "missing_password", "username", username)
		return errValidationField("password", "password is required")
	}

	user, err := a.queries.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.audit(r, "auth.login.failed", "reason", "invalid_credentials", "username", username)
			return errUnauthorized("invalid credentials")
		}
		a.audit(r, "auth.login.error", "username", username)
		return errInternal(err)
	}

	if !user.IsActive {
		a.audit(r, "auth.login.failed", "reason", "invalid_credentials", "username", username)
		return errUnauthorized("invalid credentials")
	}

	ok, err := password.Verify(req.Password, user.PasswordHash)
	if err != nil {
		a.logger.Error("password verify error", "err", err)
		a.audit(r, "auth.login.error", "username", username, "user_id", uuidString(user.ID))
		return errUnauthorized("invalid credentials")
	}
	if !ok {
		a.audit(r, "auth.login.failed", "reason", "invalid_credentials", "username", username)
		return errUnauthorized("invalid credentials")
	}

	sessionToken, tokenHash, err := newSessionToken()
	if err != nil {
		a.audit(r, "auth.login.error", "username", username, "user_id", uuidString(user.ID))
		return errInternal(err)
	}

	expiresAt := time.Now().Add(a.sessionTTL)
	if createErr := a.createSession(r.Context(), user.ID, tokenHash, expiresAt); createErr != nil {
		a.audit(r, "auth.login.error", "username", username, "user_id", uuidString(user.ID))
		return errInternal(createErr)
	}

	csrfToken, err := a.newCSRFToken()
	if err != nil {
		a.audit(r, "auth.login.error", "username", username, "user_id", uuidString(user.ID))
		return errInternal(err)
	}
	if setCSRFCookieErr := a.setCSRFCookie(w, csrfToken, expiresAt); setCSRFCookieErr != nil {
		a.audit(r, "auth.login.error", "username", username, "user_id", uuidString(user.ID))
		return errInternal(setCSRFCookieErr)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     a.sessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.sessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})
	a.audit(r, "auth.login.succeeded", "username", username, "user_id", uuidString(user.ID))
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	hadSessionCookie := false
	sessionDeleteFailed := false

	if token := a.readSessionCookie(r); token != "" {
		hadSessionCookie = true
		tokenHash := hashToken(token)
		if err := a.queries.DeleteSessionByTokenHash(r.Context(), tokenHash); err != nil {
			sessionDeleteFailed = true
			a.logger.Warn("delete session failed", "err", err)
		}
	}

	a.clearCSRFCookie(w)
	http.SetCookie(w, &http.Cookie{
		Name:     a.sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.sessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
	a.audit(r, "auth.logout", "session_cookie_present", hadSessionCookie, "session_delete_failed", sessionDeleteFailed)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	user, err := a.queries.GetUserByID(r.Context(), pgtype.UUID{Bytes: info.UserID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errUnauthorized("unauthorized")
		}
		return errInternal(err)
	}

	if !user.IsActive {
		return errUnauthorized("unauthorized")
	}

	var displayName *string
	if user.DisplayName.Valid {
		displayName = &user.DisplayName.String
	}

	if err := response.WriteJSON(w, http.StatusOK, meResponse{
		ID:          uuid.UUID(user.ID.Bytes).String(),
		Username:    user.Username,
		DisplayName: displayName,
	}); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/auth/me")
	}
	return nil
}

func (a *App) readSessionCookie(r *http.Request) string {
	c, err := r.Cookie(a.sessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func newSessionToken() (string, []byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	return token, hashToken(token), nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func (a *App) createSession(ctx context.Context, userID pgtype.UUID, tokenHash []byte, expiresAt time.Time) error {
	if !userID.Valid {
		return errors.New("invalid user id")
	}

	_, err := a.queries.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{
			Time:  expiresAt,
			Valid: true,
		},
		LastSeenAt: pgtype.Timestamptz{},
		CreatedBy:  userID,
		UpdatedBy:  userID,
	})
	return err
}

func (a *App) writeProblem(w http.ResponseWriter, status int, code, message string, details any) {
	if err := response.WriteProblem(w, status, code, message, details); err != nil {
		a.logger.Warn("write failed", "err", err, "status", status, "code", code)
	}
}
