package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

const (
	defaultItemsLimit int32 = 50
	maxItemsLimit     int32 = 200
)

// itemRequest captures item create/update payloads.
type itemRequest struct {
	Name     string  `json:"name"`
	StoreURL *string `json:"store_url"`
	AisleID  *string `json:"aisle_id"`
}

// handleItemsList returns a filtered list of items.
func (a *App) handleItemsList(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	qp := r.URL.Query()
	limit := defaultItemsLimit
	if raw := strings.TrimSpace(qp.Get("limit")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed <= 0 {
			return errValidationField("limit", "invalid limit")
		}
		if parsed > int64(maxItemsLimit) {
			return errValidationField("limit", "limit must be <= 200")
		}
		limit = int32(parsed)
	}

	query := strings.TrimSpace(qp.Get("q"))
	rows, err := a.queries.ListItems(r.Context(), sqlc.ListItemsParams{
		Q:         query,
		PageLimit: limit,
	})
	if err != nil {
		return errInternal(err)
	}

	items := make([]itemResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, itemResponseFromListRow(row))
	}

	if err := response.WriteJSON(w, http.StatusOK, items); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/items")
	}
	return nil
}

// handleItemsGet returns a single item by id.
func (a *App) handleItemsGet(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	row, err := a.queries.GetItemByID(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		return errInternal(err)
	}

	if err := response.WriteJSON(w, http.StatusOK, itemResponseFromGetRow(row)); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/items/{id}")
	}
	return nil
}

// handleItemsCreate creates a new item.
func (a *App) handleItemsCreate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	var req itemRequest
	if err := a.decodeJSON(w, r, &req); err != nil {
		return err
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return errValidationField("name", "name is required")
	}

	aisleID, err := uuidPtrToPG(req.AisleID)
	if err != nil {
		return errValidationField("aisle_id", "invalid id")
	}
	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	created, err := a.queries.CreateItem(r.Context(), sqlc.CreateItemParams{
		Name:      req.Name,
		StoreUrl:  textPtrToPG(req.StoreURL),
		AisleID:   aisleID,
		CreatedBy: userID,
		UpdatedBy: userID,
	})
	if err != nil {
		if isPGUniqueViolation(err) {
			return errValidationField("name", "name already exists")
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return errValidationField("aisle_id", "aisle does not exist")
		}
		return errInternal(err)
	}

	row, err := a.queries.GetItemByID(r.Context(), created.ID)
	if err != nil {
		return errInternal(err)
	}

	if err := response.WriteJSON(w, http.StatusOK, itemResponseFromGetRow(row)); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/items")
	}
	return nil
}

// handleItemsUpdate updates an existing item.
func (a *App) handleItemsUpdate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	var req itemRequest
	if decodeErr := a.decodeJSON(w, r, &req); decodeErr != nil {
		return decodeErr
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return errValidationField("name", "name is required")
	}

	aisleID, err := uuidPtrToPG(req.AisleID)
	if err != nil {
		return errValidationField("aisle_id", "invalid id")
	}
	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	updated, err := a.queries.UpdateItemByID(r.Context(), sqlc.UpdateItemByIDParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		Name:      req.Name,
		StoreUrl:  textPtrToPG(req.StoreURL),
		AisleID:   aisleID,
		UpdatedBy: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		if isPGUniqueViolation(err) {
			return errValidationField("name", "name already exists")
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return errValidationField("aisle_id", "aisle does not exist")
		}
		return errInternal(err)
	}

	row, err := a.queries.GetItemByID(r.Context(), updated.ID)
	if err != nil {
		return errInternal(err)
	}

	if err := response.WriteJSON(w, http.StatusOK, itemResponseFromGetRow(row)); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/items/{id}")
	}
	return nil
}

// handleItemsDelete deletes an item.
func (a *App) handleItemsDelete(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	affected, err := a.queries.DeleteItemByID(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		return errInternal(err)
	}
	if affected == 0 {
		return errNotFound()
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// itemResponseFromListRow maps a list row into an item response.
func itemResponseFromListRow(row sqlc.ListItemsRow) itemResponse {
	return buildItemResponse(
		row.ID,
		row.Name,
		row.StoreUrl,
		row.AisleID,
		row.AisleName,
		row.AisleSortGroup,
		row.AisleSortOrder,
		row.AisleNumericValue,
	)
}

// itemResponseFromGetRow maps a detail row into an item response.
func itemResponseFromGetRow(row sqlc.GetItemByIDRow) itemResponse {
	return buildItemResponse(
		row.ID,
		row.Name,
		row.StoreUrl,
		row.AisleID,
		row.AisleName,
		row.AisleSortGroup,
		row.AisleSortOrder,
		row.AisleNumericValue,
	)
}
