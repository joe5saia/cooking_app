package httpapi

import "github.com/jackc/pgx/v5/pgtype"

// groceryAisleResponse represents ordering metadata for a grocery aisle.
type groceryAisleResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SortGroup    int    `json:"sort_group"`
	SortOrder    int    `json:"sort_order"`
	NumericValue *int   `json:"numeric_value"`
}

// itemResponse exposes item details used across recipe and shopping list flows.
type itemResponse struct {
	ID       string                `json:"id"`
	Name     string                `json:"name"`
	StoreURL *string               `json:"store_url"`
	Aisle    *groceryAisleResponse `json:"aisle"`
}

// buildItemResponse constructs an item response with optional aisle details.
func buildItemResponse(
	id pgtype.UUID,
	name string,
	storeURL pgtype.Text,
	aisleID pgtype.UUID,
	aisleName pgtype.Text,
	aisleSortGroup pgtype.Int4,
	aisleSortOrder pgtype.Int4,
	aisleNumericValue pgtype.Int4,
) itemResponse {
	return itemResponse{
		ID:       uuidString(id),
		Name:     name,
		StoreURL: textStringPtr(storeURL),
		Aisle: buildAisleResponse(
			aisleID,
			aisleName,
			aisleSortGroup,
			aisleSortOrder,
			aisleNumericValue,
		),
	}
}

// buildAisleResponse maps aisle columns into a response payload.
func buildAisleResponse(
	id pgtype.UUID,
	name pgtype.Text,
	sortGroup pgtype.Int4,
	sortOrder pgtype.Int4,
	numericValue pgtype.Int4,
) *groceryAisleResponse {
	if !id.Valid {
		return nil
	}

	aisleName := ""
	if name.Valid {
		aisleName = name.String
	}
	group := int(sortGroup.Int32)
	order := int(sortOrder.Int32)
	var numeric *int
	if numericValue.Valid {
		value := int(numericValue.Int32)
		numeric = &value
	}

	return &groceryAisleResponse{
		ID:           uuidString(id),
		Name:         aisleName,
		SortGroup:    group,
		SortOrder:    order,
		NumericValue: numeric,
	}
}
