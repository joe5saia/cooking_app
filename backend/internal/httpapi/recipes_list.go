package httpapi

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

type recipeListItemResponse struct {
	ID               string              `json:"id"`
	Title            string              `json:"title"`
	Servings         int32               `json:"servings"`
	PrepTimeMinutes  int32               `json:"prep_time_minutes"`
	TotalTimeMinutes int32               `json:"total_time_minutes"`
	SourceURL        *string             `json:"source_url"`
	Notes            *string             `json:"notes"`
	RecipeBookID     *string             `json:"recipe_book_id"`
	Tags             []recipeTagResponse `json:"tags"`
	DeletedAt        *string             `json:"deleted_at"`
	UpdatedAt        string              `json:"updated_at"`
}

type recipesListResponse struct {
	Items      []recipeListItemResponse `json:"items"`
	NextCursor *string                  `json:"next_cursor"`
}

func (a *App) handleRecipesList(w http.ResponseWriter, r *http.Request) {
	if _, ok := authInfoFromRequest(r); !ok {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	qp := r.URL.Query()
	q := strings.TrimSpace(qp.Get("q"))

	includeDeleted := false
	if v := strings.TrimSpace(qp.Get("include_deleted")); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
				{Field: "include_deleted", Message: "invalid boolean"},
			})
			return
		}
		includeDeleted = parsed
	}

	bookIDStr := strings.TrimSpace(qp.Get("book_id"))
	var bookID pgtype.UUID
	if bookIDStr != "" {
		parsed, err := uuid.Parse(bookIDStr)
		if err != nil {
			a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
				{Field: "book_id", Message: "invalid id"},
			})
			return
		}
		bookID = pgtype.UUID{Bytes: parsed, Valid: true}
	}

	tagIDStr := strings.TrimSpace(qp.Get("tag_id"))
	var tagID pgtype.UUID
	if tagIDStr != "" {
		parsed, err := uuid.Parse(tagIDStr)
		if err != nil {
			a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
				{Field: "tag_id", Message: "invalid id"},
			})
			return
		}
		tagID = pgtype.UUID{Bytes: parsed, Valid: true}
	}

	limit := 50
	if v := strings.TrimSpace(qp.Get("limit")); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed <= 0 {
			a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
				{Field: "limit", Message: "invalid limit"},
			})
			return
		}
		if parsed > 200 {
			parsed = 200
		}
		limit = parsed
	}

	var cursorUpdatedAt pgtype.Timestamptz
	var cursorID pgtype.UUID
	if cursor := strings.TrimSpace(qp.Get("cursor")); cursor != "" {
		updatedAt, id, ok := parseRecipesCursor(cursor)
		if !ok {
			a.writeProblem(w, http.StatusBadRequest, "validation_error", "validation failed", []response.FieldError{
				{Field: "cursor", Message: "invalid cursor"},
			})
			return
		}
		cursorUpdatedAt = updatedAt
		cursorID = id
	}

	rows, err := a.queries.ListRecipes(r.Context(), sqlc.ListRecipesParams{
		Q:               q,
		BookID:          bookID,
		TagID:           tagID,
		IncludeDeleted:  includeDeleted,
		CursorUpdatedAt: cursorUpdatedAt,
		CursorID:        cursorID,
		PageLimit:       int32(limit + 1), //nolint:gosec // limit is bounded (<=200) above
	})
	if err != nil {
		a.logger.Error("list recipes failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	hasNext := len(rows) > limit
	if hasNext {
		rows = rows[:limit]
	}

	tagsByRecipeID := map[string][]recipeTagResponse{}
	if len(rows) > 0 {
		recipeIDs := make([]pgtype.UUID, 0, len(rows))
		for _, row := range rows {
			recipeIDs = append(recipeIDs, row.ID)
		}

		tagRows, err := a.queries.ListRecipeTagsByRecipeIDs(r.Context(), recipeIDs)
		if err != nil {
			a.logger.Error("list recipe tags failed", "err", err)
			a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
			return
		}
		for _, tr := range tagRows {
			rid := uuidString(tr.RecipeID)
			tagsByRecipeID[rid] = append(tagsByRecipeID[rid], recipeTagResponse{
				ID:   uuidString(tr.ID),
				Name: tr.Name,
			})
		}
	}

	items := make([]recipeListItemResponse, 0, len(rows))
	for _, row := range rows {
		id := uuidString(row.ID)
		tags := tagsByRecipeID[id]
		if tags == nil {
			tags = []recipeTagResponse{}
		}

		items = append(items, recipeListItemResponse{
			ID:               id,
			Title:            row.Title,
			Servings:         row.Servings,
			PrepTimeMinutes:  row.PrepTimeMinutes,
			TotalTimeMinutes: row.TotalTimeMinutes,
			SourceURL:        textStringPtr(row.SourceUrl),
			Notes:            textStringPtr(row.Notes),
			RecipeBookID:     uuidStringPtr(row.RecipeBookID),
			Tags:             tags,
			DeletedAt:        timeStringPtr(row.DeletedAt),
			UpdatedAt:        timeString(row.UpdatedAt),
		})
	}

	var nextCursor *string
	if hasNext && len(rows) > 0 {
		last := rows[len(rows)-1]
		cursor := encodeRecipesCursor(last.UpdatedAt, last.ID)
		nextCursor = &cursor
	}

	if err := response.WriteJSON(w, http.StatusOK, recipesListResponse{Items: items, NextCursor: nextCursor}); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/recipes")
	}
}

func encodeRecipesCursor(updatedAt pgtype.Timestamptz, id pgtype.UUID) string {
	if !updatedAt.Valid || !id.Valid {
		return ""
	}
	payload := strconv.FormatInt(updatedAt.Time.UTC().UnixNano(), 10) + ":" + uuidString(id)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func parseRecipesCursor(cursor string) (pgtype.Timestamptz, pgtype.UUID, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}, false
	}
	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 {
		return pgtype.Timestamptz{}, pgtype.UUID{}, false
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}, false
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}, false
	}
	return pgtype.Timestamptz{Time: time.Unix(0, nanos).UTC(), Valid: true}, pgtype.UUID{Bytes: id, Valid: true}, true
}
