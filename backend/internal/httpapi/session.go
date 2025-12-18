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

func (a *App) requireSessionUser(r *http.Request) (*sessionUser, error) {
	token := a.readSessionCookie(r)
	if token == "" {
		return nil, errUnauthorized("unauthorized")
	}

	tokenHash := hashToken(token)
	row, err := a.queries.GetSessionUserByTokenHash(r.Context(), tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errUnauthorized("unauthorized")
		}
		return nil, errInternal(err)
	}

	if !row.SessionExpiresAt.Valid || time.Now().After(row.SessionExpiresAt.Time) {
		return nil, errUnauthorized("unauthorized")
	}
	if !row.IsActive || !row.UserID.Valid {
		return nil, errUnauthorized("unauthorized")
	}

	return &sessionUser{
		UserID:      row.UserID,
		Username:    row.Username,
		DisplayName: row.DisplayName,
	}, nil
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
