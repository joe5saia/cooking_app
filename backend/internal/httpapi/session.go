package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
)

type sessionUser struct {
	UserID      pgtype.UUID
	Username    string
	DisplayName pgtype.Text
}

func (a *App) requireSessionUser(w http.ResponseWriter, r *http.Request) (*sessionUser, bool) {
	token := a.readSessionCookie(r)
	if token == "" {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return nil, false
	}

	tokenHash := hashToken(token)
	row, err := a.queries.GetSessionUserByTokenHash(r.Context(), tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
			return nil, false
		}
		a.logger.Error("get session failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return nil, false
	}

	if !row.SessionExpiresAt.Valid || time.Now().After(row.SessionExpiresAt.Time) {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return nil, false
	}
	if !row.IsActive || !row.UserID.Valid {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return nil, false
	}

	return &sessionUser{
		UserID:      row.UserID,
		Username:    row.Username,
		DisplayName: row.DisplayName,
	}, true
}

func patListResponse(tokens []sqlc.PersonalAccessToken) []map[string]any {
	out := make([]map[string]any, 0, len(tokens))
	for _, t := range tokens {
		item := map[string]any{
			"id":         uuidString(t.ID),
			"name":       t.Name,
			"created_at": timeString(t.CreatedAt),
		}
		if t.LastUsedAt.Valid {
			item["last_used_at"] = timeString(t.LastUsedAt)
		} else {
			item["last_used_at"] = nil
		}
		if t.ExpiresAt.Valid {
			item["expires_at"] = timeString(t.ExpiresAt)
		} else {
			item["expires_at"] = nil
		}
		out = append(out, item)
	}
	return out
}

func uuidString(v pgtype.UUID) string {
	if !v.Valid {
		return ""
	}
	return uuid.UUID(v.Bytes).String()
}

func timeString(v pgtype.Timestamptz) string {
	if !v.Valid {
		return ""
	}
	return v.Time.UTC().Format(time.RFC3339Nano)
}

func (a *App) writeValidation(w http.ResponseWriter, field, message string) {
	a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []map[string]string{
		{"field": field, "message": message},
	})
}
