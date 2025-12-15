package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeProblem(w, http.StatusBadRequest, "bad_request", "invalid JSON", nil)
		return
	}

	username, err := users.NormalizeUsername(req.Username)
	if err != nil {
		a.writeProblem(w, http.StatusBadRequest, "validation_error", err.Error(), []response.FieldError{
			{Field: "username", Message: err.Error()},
		})
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		a.writeProblem(w, http.StatusBadRequest, "validation_error", "password is required", []response.FieldError{
			{Field: "password", Message: "password is required"},
		})
		return
	}

	user, err := a.queries.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "invalid credentials", nil)
			return
		}
		a.logger.Error("get user failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	if !user.IsActive {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "invalid credentials", nil)
		return
	}

	ok, err := password.Verify(req.Password, user.PasswordHash)
	if err != nil {
		a.logger.Error("password verify error", "err", err)
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "invalid credentials", nil)
		return
	}
	if !ok {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "invalid credentials", nil)
		return
	}

	sessionToken, tokenHash, err := newSessionToken()
	if err != nil {
		a.logger.Error("session token generation failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	expiresAt := time.Now().Add(a.sessionTTL)
	if err := a.createSession(r.Context(), user.ID, tokenHash, expiresAt); err != nil {
		a.logger.Error("create session failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
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
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if token := a.readSessionCookie(r); token != "" {
		tokenHash := hashToken(token)
		if err := a.queries.DeleteSessionByTokenHash(r.Context(), tokenHash); err != nil {
			a.logger.Warn("delete session failed", "err", err)
		}
	}

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
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	token := a.readSessionCookie(r)
	if token == "" {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	tokenHash := hashToken(token)

	row, err := a.queries.GetSessionUserByTokenHash(r.Context(), tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
			return
		}
		a.logger.Error("get session failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	if !row.SessionExpiresAt.Valid || time.Now().After(row.SessionExpiresAt.Time) {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	if !row.IsActive {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	if !row.UserID.Valid {
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	var displayName *string
	if row.DisplayName.Valid {
		displayName = &row.DisplayName.String
	}

	if err := response.WriteJSON(w, http.StatusOK, meResponse{
		ID:          uuid.UUID(row.UserID.Bytes).String(),
		Username:    row.Username,
		DisplayName: displayName,
	}); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/auth/me")
	}
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
