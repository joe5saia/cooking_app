package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

type shoppingListRequest struct {
	ListDate string  `json:"list_date"`
	Name     string  `json:"name"`
	Notes    *string `json:"notes"`
}

// shoppingListResponse represents a shopping list without its items.
type shoppingListResponse struct {
	ID        string  `json:"id"`
	ListDate  string  `json:"list_date"`
	Name      string  `json:"name"`
	Notes     *string `json:"notes"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// shoppingListDetailResponse includes a shopping list and its items.
type shoppingListDetailResponse struct {
	ID        string                     `json:"id"`
	ListDate  string                     `json:"list_date"`
	Name      string                     `json:"name"`
	Notes     *string                    `json:"notes"`
	Items     []shoppingListItemResponse `json:"items"`
	CreatedAt string                     `json:"created_at"`
	UpdatedAt string                     `json:"updated_at"`
}

// shoppingListItemResponse represents a shopping list item with live item details.
type shoppingListItemResponse struct {
	ID           string       `json:"id"`
	Item         itemResponse `json:"item"`
	Quantity     *float64     `json:"quantity"`
	QuantityText *string      `json:"quantity_text"`
	Unit         *string      `json:"unit"`
	IsPurchased  bool         `json:"is_purchased"`
	PurchasedAt  *string      `json:"purchased_at"`
}

type shoppingListItemInput struct {
	ItemID       string   `json:"item_id"`
	Quantity     *float64 `json:"quantity"`
	QuantityText *string  `json:"quantity_text"`
	Unit         *string  `json:"unit"`
}

type shoppingListItemsAddRequest struct {
	Items []shoppingListItemInput `json:"items"`
}

type shoppingListRecipesAddRequest struct {
	RecipeIDs []string `json:"recipe_ids"`
}

type shoppingListMealPlanAddRequest struct {
	Date string `json:"date"`
}

type shoppingListItemPurchaseRequest struct {
	IsPurchased bool `json:"is_purchased"`
}

// handleShoppingListsList returns shopping lists within a date range.
func (a *App) handleShoppingListsList(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	qp := r.URL.Query()
	start, err := parseMealPlanDate("start", qp.Get("start"))
	if err != nil {
		return err
	}
	end, err := parseMealPlanDate("end", qp.Get("end"))
	if err != nil {
		return err
	}
	if start.Time.After(end.Time) {
		return errValidationField("end", "end must be on or after start")
	}

	rows, err := a.queries.ListShoppingListsByDateRange(r.Context(), sqlc.ListShoppingListsByDateRangeParams{
		UserID:    pgtype.UUID{Bytes: info.UserID, Valid: true},
		StartDate: start,
		EndDate:   end,
	})
	if err != nil {
		return errInternal(err)
	}

	out := make([]shoppingListResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, shoppingListResponseFromListRow(row))
	}

	if err := response.WriteJSON(w, http.StatusOK, out); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/shopping-lists")
	}
	return nil
}

// handleShoppingListsCreate creates a new shopping list.
func (a *App) handleShoppingListsCreate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	var req shoppingListRequest
	if err := a.decodeJSON(w, r, &req); err != nil {
		return err
	}

	listDate, err := parseMealPlanDate("list_date", req.ListDate)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return errValidationField("name", "name is required")
	}

	userID := pgtype.UUID{Bytes: info.UserID, Valid: true}
	row, err := a.queries.CreateShoppingList(r.Context(), sqlc.CreateShoppingListParams{
		ListDate:  listDate,
		Name:      name,
		Notes:     textPtrToPG(req.Notes),
		CreatedBy: userID,
		UpdatedBy: userID,
	})
	if err != nil {
		return errInternal(err)
	}

	if err := response.WriteJSON(w, http.StatusCreated, shoppingListResponseFromCreateRow(row)); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/shopping-lists")
	}
	return nil
}

// handleShoppingListsGet returns a shopping list and its items.
func (a *App) handleShoppingListsGet(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	ctx := r.Context()
	row, err := a.queries.GetShoppingListByID(ctx, sqlc.GetShoppingListByIDParams{
		ID:     pgtype.UUID{Bytes: id, Valid: true},
		UserID: pgtype.UUID{Bytes: info.UserID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		return errInternal(err)
	}

	items, err := a.loadShoppingListItems(ctx, pgtype.UUID{Bytes: id, Valid: true}, info.UserID)
	if err != nil {
		return err
	}

	resp := shoppingListDetailResponse{
		ID:        uuidString(row.ID),
		ListDate:  mealPlanDateString(row.ListDate),
		Name:      row.Name,
		Notes:     textStringPtr(row.Notes),
		Items:     items,
		CreatedAt: timeString(row.CreatedAt),
		UpdatedAt: timeString(row.UpdatedAt),
	}

	if err := response.WriteJSON(w, http.StatusOK, resp); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/shopping-lists/{id}")
	}
	return nil
}

// handleShoppingListsUpdate updates a shopping list.
func (a *App) handleShoppingListsUpdate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	var req shoppingListRequest
	if decodeErr := a.decodeJSON(w, r, &req); decodeErr != nil {
		return decodeErr
	}

	listDate, err := parseMealPlanDate("list_date", req.ListDate)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return errValidationField("name", "name is required")
	}

	row, err := a.queries.UpdateShoppingListByID(r.Context(), sqlc.UpdateShoppingListByIDParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		UserID:    pgtype.UUID{Bytes: info.UserID, Valid: true},
		ListDate:  listDate,
		Name:      name,
		Notes:     textPtrToPG(req.Notes),
		UpdatedBy: pgtype.UUID{Bytes: info.UserID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		return errInternal(err)
	}

	if err := response.WriteJSON(w, http.StatusOK, shoppingListResponseFromUpdateRow(row)); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/shopping-lists/{id}")
	}
	return nil
}

// handleShoppingListsDelete deletes a shopping list and its items.
func (a *App) handleShoppingListsDelete(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	affected, err := a.queries.DeleteShoppingListByID(r.Context(), sqlc.DeleteShoppingListByIDParams{
		ID:     pgtype.UUID{Bytes: id, Valid: true},
		UserID: pgtype.UUID{Bytes: info.UserID, Valid: true},
	})
	if err != nil {
		return errInternal(err)
	}
	if affected == 0 {
		return errNotFound()
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// handleShoppingListItemsList lists items for a shopping list.
func (a *App) handleShoppingListItemsList(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	listID, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	items, err := a.loadShoppingListItems(r.Context(), pgtype.UUID{Bytes: listID, Valid: true}, info.UserID)
	if err != nil {
		return err
	}

	if err := response.WriteJSON(w, http.StatusOK, items); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/shopping-lists/{id}/items")
	}
	return nil
}

// handleShoppingListItemsAdd adds explicit items to a shopping list.
func (a *App) handleShoppingListItemsAdd(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	listID, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	var req shoppingListItemsAddRequest
	if err = a.decodeJSON(w, r, &req); err != nil {
		return err
	}
	if len(req.Items) == 0 {
		return errValidationField("items", "items is required")
	}

	items, err := normalizeShoppingListItemInputs(req.Items)
	if err != nil {
		return err
	}

	if err = a.upsertShoppingListItems(r.Context(), pgtype.UUID{Bytes: listID, Valid: true}, info.UserID, items); err != nil {
		return err
	}

	out, err := a.loadShoppingListItems(r.Context(), pgtype.UUID{Bytes: listID, Valid: true}, info.UserID)
	if err != nil {
		return err
	}

	if err = response.WriteJSON(w, http.StatusOK, out); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/shopping-lists/{id}/items")
	}
	return nil
}

// handleShoppingListItemsAddFromRecipes adds items from recipes to a shopping list.
func (a *App) handleShoppingListItemsAddFromRecipes(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	listID, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	var req shoppingListRecipesAddRequest
	if err = a.decodeJSON(w, r, &req); err != nil {
		return err
	}
	if len(req.RecipeIDs) == 0 {
		return errValidationField("recipe_ids", "recipe_ids is required")
	}

	recipeIDs, err := uuidsToPG(req.RecipeIDs)
	if err != nil {
		return errValidationField("recipe_ids", "invalid id")
	}

	rows, err := a.queries.ListRecipeIngredientsByRecipeIDs(r.Context(), recipeIDs)
	if err != nil {
		return errInternal(err)
	}

	items, err := aggregateRecipeIngredients(rows)
	if err != nil {
		return err
	}

	if err = a.upsertShoppingListItems(r.Context(), pgtype.UUID{Bytes: listID, Valid: true}, info.UserID, items); err != nil {
		return err
	}

	out, err := a.loadShoppingListItems(r.Context(), pgtype.UUID{Bytes: listID, Valid: true}, info.UserID)
	if err != nil {
		return err
	}

	if err = response.WriteJSON(w, http.StatusOK, out); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/shopping-lists/{id}/items/from-recipes")
	}
	return nil
}

// handleShoppingListItemsAddFromMealPlan adds items from a meal plan date to a shopping list.
func (a *App) handleShoppingListItemsAddFromMealPlan(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	listID, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	var req shoppingListMealPlanAddRequest
	if err = a.decodeJSON(w, r, &req); err != nil {
		return err
	}

	planDate, err := parseMealPlanDate("date", req.Date)
	if err != nil {
		return err
	}

	rows, err := a.queries.ListRecipeIngredientsByMealPlanDate(r.Context(), sqlc.ListRecipeIngredientsByMealPlanDateParams{
		UserID:   pgtype.UUID{Bytes: info.UserID, Valid: true},
		PlanDate: planDate,
	})
	if err != nil {
		return errInternal(err)
	}

	items, err := aggregateMealPlanIngredients(rows)
	if err != nil {
		return err
	}

	if err = a.upsertShoppingListItems(r.Context(), pgtype.UUID{Bytes: listID, Valid: true}, info.UserID, items); err != nil {
		return err
	}

	out, err := a.loadShoppingListItems(r.Context(), pgtype.UUID{Bytes: listID, Valid: true}, info.UserID)
	if err != nil {
		return err
	}

	if err = response.WriteJSON(w, http.StatusOK, out); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/shopping-lists/{id}/items/from-meal-plan")
	}
	return nil
}

// handleShoppingListItemsUpdate updates purchase state for a shopping list item.
func (a *App) handleShoppingListItemsUpdate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	listID, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}
	itemID, err := parseUUIDParam(r, "item_id")
	if err != nil {
		return err
	}

	var req shoppingListItemPurchaseRequest
	if decodeErr := a.decodeJSON(w, r, &req); decodeErr != nil {
		return decodeErr
	}

	row, err := a.queries.UpdateShoppingListItemPurchased(r.Context(), sqlc.UpdateShoppingListItemPurchasedParams{
		ID:             pgtype.UUID{Bytes: itemID, Valid: true},
		ShoppingListID: pgtype.UUID{Bytes: listID, Valid: true},
		UserID:         pgtype.UUID{Bytes: info.UserID, Valid: true},
		IsPurchased:    req.IsPurchased,
		UpdatedBy:      pgtype.UUID{Bytes: info.UserID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		return errInternal(err)
	}

	resp, err := shoppingListItemResponseFromUpdateRow(row)
	if err != nil {
		return err
	}

	if err := response.WriteJSON(w, http.StatusOK, resp); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/shopping-lists/{id}/items/{item_id}")
	}
	return nil
}

// handleShoppingListItemsDelete deletes a shopping list item.
func (a *App) handleShoppingListItemsDelete(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	listID, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}
	itemID, err := parseUUIDParam(r, "item_id")
	if err != nil {
		return err
	}

	affected, err := a.queries.DeleteShoppingListItemByID(r.Context(), sqlc.DeleteShoppingListItemByIDParams{
		ID:             pgtype.UUID{Bytes: itemID, Valid: true},
		ShoppingListID: pgtype.UUID{Bytes: listID, Valid: true},
		UserID:         pgtype.UUID{Bytes: info.UserID, Valid: true},
	})
	if err != nil {
		return errInternal(err)
	}
	if affected == 0 {
		return errNotFound()
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// shoppingListResponseFromRow maps a SQL row to a shopping list response.
func shoppingListResponseFromBase(
	id pgtype.UUID,
	listDate pgtype.Date,
	name string,
	notes pgtype.Text,
	createdAt pgtype.Timestamptz,
	updatedAt pgtype.Timestamptz,
) shoppingListResponse {
	return shoppingListResponse{
		ID:        uuidString(id),
		ListDate:  mealPlanDateString(listDate),
		Name:      name,
		Notes:     textStringPtr(notes),
		CreatedAt: timeString(createdAt),
		UpdatedAt: timeString(updatedAt),
	}
}

// shoppingListResponseFromListRow maps list rows to responses.
func shoppingListResponseFromListRow(row sqlc.ListShoppingListsByDateRangeRow) shoppingListResponse {
	return shoppingListResponseFromBase(row.ID, row.ListDate, row.Name, row.Notes, row.CreatedAt, row.UpdatedAt)
}

// shoppingListResponseFromCreateRow maps create rows to responses.
func shoppingListResponseFromCreateRow(row sqlc.CreateShoppingListRow) shoppingListResponse {
	return shoppingListResponseFromBase(row.ID, row.ListDate, row.Name, row.Notes, row.CreatedAt, row.UpdatedAt)
}

// shoppingListResponseFromUpdateRow maps update rows to responses.
func shoppingListResponseFromUpdateRow(row sqlc.UpdateShoppingListByIDRow) shoppingListResponse {
	return shoppingListResponseFromBase(row.ID, row.ListDate, row.Name, row.Notes, row.CreatedAt, row.UpdatedAt)
}

// loadShoppingListItems fetches shopping list items with item details.
func (a *App) loadShoppingListItems(ctx context.Context, listID pgtype.UUID, userID uuid.UUID) ([]shoppingListItemResponse, error) {
	rows, err := a.queries.ListShoppingListItemsByListID(ctx, sqlc.ListShoppingListItemsByListIDParams{
		ShoppingListID: listID,
		UserID:         pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil {
		return nil, errInternal(err)
	}

	items := make([]shoppingListItemResponse, 0, len(rows))
	for _, row := range rows {
		resp, buildErr := shoppingListItemResponseFromListRow(row)
		if buildErr != nil {
			return nil, buildErr
		}
		items = append(items, resp)
	}

	return items, nil
}

// shoppingListItemResponseFromListRow maps list rows into responses.
func shoppingListItemResponseFromListRow(row sqlc.ListShoppingListItemsByListIDRow) (shoppingListItemResponse, error) {
	quantity, err := float64PtrFromNumeric(row.Quantity)
	if err != nil {
		return shoppingListItemResponse{}, err
	}

	return shoppingListItemResponse{
		ID:           uuidString(row.ID),
		Quantity:     quantity,
		QuantityText: textStringPtr(row.QuantityText),
		Unit:         textStringPtr(row.Unit),
		IsPurchased:  row.IsPurchased,
		PurchasedAt:  timeStringPtr(row.PurchasedAt),
		Item: buildItemResponse(
			row.ItemID,
			row.ItemName,
			row.ItemStoreUrl,
			row.ItemAisleID,
			row.AisleName,
			row.AisleSortGroup,
			row.AisleSortOrder,
			row.AisleNumericValue,
		),
	}, nil
}

// shoppingListItemResponseFromUpdateRow maps update rows into responses.
func shoppingListItemResponseFromUpdateRow(row sqlc.UpdateShoppingListItemPurchasedRow) (shoppingListItemResponse, error) {
	quantity, err := float64PtrFromNumeric(row.Quantity)
	if err != nil {
		return shoppingListItemResponse{}, err
	}

	aisleName := pgtype.Text{String: row.AisleName, Valid: row.ItemAisleID.Valid}
	aisleSortGroup := pgtype.Int4{Int32: row.AisleSortGroup, Valid: row.ItemAisleID.Valid}
	aisleSortOrder := pgtype.Int4{Int32: row.AisleSortOrder, Valid: row.ItemAisleID.Valid}

	return shoppingListItemResponse{
		ID:           uuidString(row.ID),
		Quantity:     quantity,
		QuantityText: textStringPtr(row.QuantityText),
		Unit:         textStringPtr(row.Unit),
		IsPurchased:  row.IsPurchased,
		PurchasedAt:  timeStringPtr(row.PurchasedAt),
		Item: buildItemResponse(
			row.ItemID,
			row.ItemName,
			row.ItemStoreUrl,
			row.ItemAisleID,
			aisleName,
			aisleSortGroup,
			aisleSortOrder,
			row.AisleNumericValue,
		),
	}, nil
}

// float64PtrFromNumeric converts a PG numeric into a float pointer.
func float64PtrFromNumeric(value pgtype.Numeric) (*float64, error) {
	if !value.Valid {
		return nil, nil
	}
	floatVal, err := value.Float64Value()
	if err != nil {
		return nil, err
	}
	if !floatVal.Valid {
		return nil, nil
	}
	v := floatVal.Float64
	return &v, nil
}

// normalizedShoppingListItem stores validated list item input for inserts.
type normalizedShoppingListItem struct {
	itemID       pgtype.UUID
	quantity     *float64
	quantityText *string
	unit         *string
}

// normalizeShoppingListItemInputs validates list item inputs.
func normalizeShoppingListItemInputs(inputs []shoppingListItemInput) ([]normalizedShoppingListItem, error) {
	items := make([]normalizedShoppingListItem, 0, len(inputs))
	for _, input := range inputs {
		itemID, err := parseShoppingListItemID(input.ItemID)
		if err != nil {
			return nil, err
		}

		unit := trimPtr(input.Unit)
		quantityText := trimPtr(input.QuantityText)
		if input.Quantity == nil && unit == nil && quantityText != nil && containsLetters(*quantityText) {
			unit = quantityText
			quantityText = nil
		}

		items = append(items, normalizedShoppingListItem{
			itemID:       itemID,
			quantity:     input.Quantity,
			quantityText: quantityText,
			unit:         unit,
		})
	}
	return items, nil
}

// aggregateRecipeIngredients aggregates ingredients into list items.
func aggregateRecipeIngredients(rows []sqlc.ListRecipeIngredientsByRecipeIDsRow) ([]normalizedShoppingListItem, error) {
	return aggregateRecipeIngredientRows(convertRecipeIngredientRows(rows))
}

// aggregateMealPlanIngredients aggregates meal plan ingredients into list items.
func aggregateMealPlanIngredients(rows []sqlc.ListRecipeIngredientsByMealPlanDateRow) ([]normalizedShoppingListItem, error) {
	return aggregateRecipeIngredientRows(convertMealPlanIngredientRows(rows))
}

// aggregateRecipeIngredientRows aggregates ingredients into list items.
func aggregateRecipeIngredientRows(rows []ingredientRow) ([]normalizedShoppingListItem, error) {
	items := make(map[string]*aggregatedListItem)
	for _, row := range rows {
		quantity, err := float64PtrFromNumeric(row.quantity)
		if err != nil {
			return nil, errInternal(err)
		}

		unit := trimPtr(textStringPtr(row.unit))
		quantityText := trimPtr(textStringPtr(row.quantityText))
		if quantity == nil && unit == nil && quantityText != nil && containsLetters(*quantityText) {
			unit = quantityText
			quantityText = nil
		}
		key := uuidString(row.itemID)
		unitKey := ""
		if unit != nil {
			unitKey = strings.ToLower(*unit)
		}
		mapKey := key + "|" + unitKey
		current, ok := items[mapKey]
		if !ok {
			items[mapKey] = &aggregatedListItem{
				itemID:       row.itemID,
				quantity:     quantity,
				quantityText: quantityText,
				unit:         unit,
				count:        1,
				hasNumeric:   quantity != nil,
			}
			continue
		}

		current.count++
		if quantity != nil {
			current.hasNumeric = true
			if current.quantity == nil {
				current.quantity = quantity
			} else {
				*current.quantity += *quantity
			}
		}
		if current.unit == nil {
			current.unit = unit
		}
		if current.quantityText == nil {
			current.quantityText = quantityText
		}
	}

	out := make([]normalizedShoppingListItem, 0, len(items))
	for _, item := range items {
		quantityText := item.quantityText
		if item.hasNumeric && item.count > 1 {
			quantityText = nil
		}
		out = append(out, normalizedShoppingListItem{
			itemID:       item.itemID,
			quantity:     item.quantity,
			quantityText: quantityText,
			unit:         item.unit,
		})
	}
	return out, nil
}

// ingredientRow is a lightweight adapter for ingredient aggregations.
type ingredientRow struct {
	itemID       pgtype.UUID
	quantity     pgtype.Numeric
	quantityText pgtype.Text
	unit         pgtype.Text
}

type aggregatedListItem struct {
	itemID       pgtype.UUID
	quantity     *float64
	quantityText *string
	unit         *string
	count        int
	hasNumeric   bool
}

// convertRecipeIngredientRows converts sqlc recipe ingredient rows.
func convertRecipeIngredientRows(rows []sqlc.ListRecipeIngredientsByRecipeIDsRow) []ingredientRow {
	out := make([]ingredientRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ingredientRow{
			itemID:       row.ItemID,
			quantity:     row.Quantity,
			quantityText: row.QuantityText,
			unit:         row.Unit,
		})
	}
	return out
}

// convertMealPlanIngredientRows converts sqlc meal plan ingredient rows.
func convertMealPlanIngredientRows(rows []sqlc.ListRecipeIngredientsByMealPlanDateRow) []ingredientRow {
	out := make([]ingredientRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, ingredientRow{
			itemID:       row.ItemID,
			quantity:     row.Quantity,
			quantityText: row.QuantityText,
			unit:         row.Unit,
		})
	}
	return out
}

// upsertShoppingListItems inserts or updates list items in a transaction.
func (a *App) upsertShoppingListItems(ctx context.Context, listID pgtype.UUID, userID uuid.UUID, items []normalizedShoppingListItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return errInternal(err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			a.logger.Warn("rollback failed", "err", rollbackErr)
		}
	}()

	queries := a.queries.WithTx(tx)
	actor := pgtype.UUID{Bytes: userID, Valid: true}
	for _, item := range items {
		quantity, err := numericPtrFromFloat64(item.quantity)
		if err != nil {
			return errValidationField("quantity", "invalid quantity")
		}

		_, err = queries.UpsertShoppingListItem(ctx, sqlc.UpsertShoppingListItemParams{
			ShoppingListID: listID,
			UserID:         actor,
			ItemID:         item.itemID,
			Unit:           textPtrToPG(item.unit),
			Quantity:       quantity,
			QuantityText:   textPtrToPG(item.quantityText),
			CreatedBy:      actor,
			UpdatedBy:      actor,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errNotFound()
			}
			if isPGUniqueViolation(err) {
				return errValidationField("items", "duplicate item entry")
			}
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" {
				return errValidationField("item_id", "item does not exist")
			}
			return errInternal(err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return errInternal(err)
	}
	return nil
}

// parseShoppingListItemID validates an item id.
func parseShoppingListItemID(value string) (pgtype.UUID, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return pgtype.UUID{}, errValidationField("item_id", "item_id is required")
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return pgtype.UUID{}, errValidationField("item_id", "invalid id")
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

// trimPtr trims a string pointer and normalizes empty values to nil.
func trimPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// containsLetters reports whether a string contains any unicode letters.
func containsLetters(value string) bool {
	return strings.IndexFunc(value, unicode.IsLetter) >= 0
}
