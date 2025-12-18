package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

type createTagRequest struct {
	Name string `json:"name"`
}

type updateTagRequest struct {
	Name string `json:"name"`
}

type tagResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

func (a *App) handleTagsList(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	tags, err := a.queries.ListTags(r.Context())
	if err != nil {
		return errInternal(err)
	}

	out := make([]tagResponse, 0, len(tags))
	for _, t := range tags {
		out = append(out, tagResponse{
			ID:        uuidString(t.ID),
			Name:      t.Name,
			CreatedAt: timeString(t.CreatedAt),
		})
	}

	if err := response.WriteJSON(w, http.StatusOK, out); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/tags")
	}
	return nil
}

func (a *App) handleTagsCreate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	var req createTagRequest
	if err := a.decodeJSON(w, r, &req); err != nil {
		return err
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return errValidationField("name", "name is required")
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	row, err := a.queries.CreateTag(r.Context(), sqlc.CreateTagParams{
		Name:      req.Name,
		CreatedBy: userID,
		UpdatedBy: userID,
	})
	if err != nil {
		if isPGUniqueViolation(err) {
			return errValidationField("name", "name already exists")
		}
		return errInternal(err)
	}

	resp := tagResponse{
		ID:        uuidString(row.ID),
		Name:      row.Name,
		CreatedAt: timeString(row.CreatedAt),
	}

	if err := response.WriteJSON(w, http.StatusOK, resp); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/tags")
	}
	return nil
}

func (a *App) handleTagsUpdate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	var req updateTagRequest
	if decodeErr := a.decodeJSON(w, r, &req); decodeErr != nil {
		return decodeErr
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return errValidationField("name", "name is required")
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	row, err := a.queries.UpdateTagByID(r.Context(), sqlc.UpdateTagByIDParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		Name:      req.Name,
		UpdatedBy: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		if isPGUniqueViolation(err) {
			return errValidationField("name", "name already exists")
		}
		return errInternal(err)
	}

	resp := tagResponse{
		ID:        uuidString(row.ID),
		Name:      row.Name,
		CreatedAt: timeString(row.CreatedAt),
	}

	if err := response.WriteJSON(w, http.StatusOK, resp); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/tags")
	}
	return nil
}

func (a *App) handleTagsDelete(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	affected, err := a.queries.DeleteTagByID(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		return errInternal(err)
	}
	if affected == 0 {
		return errNotFound()
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
