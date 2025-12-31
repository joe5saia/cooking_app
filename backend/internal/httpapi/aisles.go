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

// aisleRequest captures aisle create/update payloads.
type aisleRequest struct {
	Name         string `json:"name"`
	SortGroup    int    `json:"sort_group"`
	SortOrder    int    `json:"sort_order"`
	NumericValue *int   `json:"numeric_value"`
}

// handleAislesList lists all grocery aisles in sort order.
func (a *App) handleAislesList(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	rows, err := a.queries.ListGroceryAisles(r.Context())
	if err != nil {
		return errInternal(err)
	}

	aisles := make([]groceryAisleResponse, 0, len(rows))
	for _, row := range rows {
		aisles = append(aisles, aisleResponseFromRow(row))
	}

	if err := response.WriteJSON(w, http.StatusOK, aisles); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/aisles")
	}
	return nil
}

// handleAislesGet returns a grocery aisle by id.
func (a *App) handleAislesGet(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	row, err := a.queries.GetGroceryAisleByID(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		return errInternal(err)
	}

	if err := response.WriteJSON(w, http.StatusOK, aisleResponseFromRow(row)); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/aisles/{id}")
	}
	return nil
}

// handleAislesCreate creates a grocery aisle.
func (a *App) handleAislesCreate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	var req aisleRequest
	if err := a.decodeJSON(w, r, &req); err != nil {
		return err
	}

	parsed, err := normalizeAisleRequest(req)
	if err != nil {
		return err
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	row, err := a.queries.CreateGroceryAisle(r.Context(), sqlc.CreateGroceryAisleParams{
		Name:         parsed.name,
		SortGroup:    parsed.sortGroup,
		SortOrder:    parsed.sortOrder,
		NumericValue: parsed.numericValue,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	})
	if err != nil {
		if isPGUniqueViolation(err) {
			return errValidationField("name", "name already exists")
		}
		return errInternal(err)
	}

	if err := response.WriteJSON(w, http.StatusOK, aisleResponseFromRow(row)); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/aisles")
	}
	return nil
}

// handleAislesUpdate updates a grocery aisle.
func (a *App) handleAislesUpdate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	var req aisleRequest
	if decodeErr := a.decodeJSON(w, r, &req); decodeErr != nil {
		return decodeErr
	}

	parsed, err := normalizeAisleRequest(req)
	if err != nil {
		return err
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	row, err := a.queries.UpdateGroceryAisleByID(r.Context(), sqlc.UpdateGroceryAisleByIDParams{
		ID:           pgtype.UUID{Bytes: id, Valid: true},
		Name:         parsed.name,
		SortGroup:    parsed.sortGroup,
		SortOrder:    parsed.sortOrder,
		NumericValue: parsed.numericValue,
		UpdatedBy:    userID,
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

	if err := response.WriteJSON(w, http.StatusOK, aisleResponseFromRow(row)); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/aisles/{id}")
	}
	return nil
}

// handleAislesDelete deletes a grocery aisle.
func (a *App) handleAislesDelete(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	affected, err := a.queries.DeleteGroceryAisleByID(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		return errInternal(err)
	}
	if affected == 0 {
		return errNotFound()
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

type normalizedAisleRequest struct {
	name         string
	sortGroup    int32
	sortOrder    int32
	numericValue pgtype.Int4
}

// normalizeAisleRequest validates and converts aisle fields for persistence.
func normalizeAisleRequest(req aisleRequest) (normalizedAisleRequest, error) {
	trimmedName := strings.TrimSpace(req.Name)
	if trimmedName == "" {
		return normalizedAisleRequest{}, errValidationField("name", "name is required")
	}
	if req.SortGroup < 0 || req.SortGroup > 2 {
		return normalizedAisleRequest{}, errValidationField("sort_group", "sort_group must be 0, 1, or 2")
	}
	if req.SortOrder < 0 {
		return normalizedAisleRequest{}, errValidationField("sort_order", "sort_order must be >= 0")
	}

	sortGroup, ok := intToInt32Checked(req.SortGroup)
	if !ok {
		return normalizedAisleRequest{}, errValidationField("sort_group", "sort_group is too large")
	}
	sortOrder, ok := intToInt32Checked(req.SortOrder)
	if !ok {
		return normalizedAisleRequest{}, errValidationField("sort_order", "sort_order is too large")
	}

	var numeric pgtype.Int4
	if req.NumericValue != nil {
		if *req.NumericValue < 0 {
			return normalizedAisleRequest{}, errValidationField("numeric_value", "numeric_value must be >= 0")
		}
		value, ok := intToInt32Checked(*req.NumericValue)
		if !ok {
			return normalizedAisleRequest{}, errValidationField("numeric_value", "numeric_value is too large")
		}
		numeric = pgtype.Int4{Int32: value, Valid: true}
	}

	return normalizedAisleRequest{
		name:         trimmedName,
		sortGroup:    sortGroup,
		sortOrder:    sortOrder,
		numericValue: numeric,
	}, nil
}

// aisleResponseFromRow maps a grocery aisle row into a response.
func aisleResponseFromRow(row sqlc.GroceryAisle) groceryAisleResponse {
	var numericValue *int
	if row.NumericValue.Valid {
		value := int(row.NumericValue.Int32)
		numericValue = &value
	}
	return groceryAisleResponse{
		ID:           uuidString(row.ID),
		Name:         row.Name,
		SortGroup:    int(row.SortGroup),
		SortOrder:    int(row.SortOrder),
		NumericValue: numericValue,
	}
}
