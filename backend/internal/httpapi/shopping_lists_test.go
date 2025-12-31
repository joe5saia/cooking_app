package httpapi_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/bootstrap"
	"github.com/saiaj/cooking_app/backend/internal/config"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi"
	"github.com/saiaj/cooking_app/backend/internal/logging"
	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

type testShoppingListResponse struct {
	ID        string  `json:"id"`
	ListDate  string  `json:"list_date"`
	Name      string  `json:"name"`
	Notes     *string `json:"notes"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type testShoppingListDetailResponse struct {
	ID        string                 `json:"id"`
	ListDate  string                 `json:"list_date"`
	Name      string                 `json:"name"`
	Notes     *string                `json:"notes"`
	Items     []testShoppingListItem `json:"items"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
}

type testShoppingListItem struct {
	ID           string           `json:"id"`
	Item         testItemResponse `json:"item"`
	Quantity     *float64         `json:"quantity"`
	QuantityText *string          `json:"quantity_text"`
	Unit         *string          `json:"unit"`
	IsPurchased  bool             `json:"is_purchased"`
	PurchasedAt  *string          `json:"purchased_at"`
}

func TestShoppingLists_CRUD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)

	_, userErr := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	})
	if userErr != nil {
		t.Fatalf("bootstrap user: %v", userErr)
	}

	app, err := httpapi.New(ctx, logging.New("error"), config.Config{
		DatabaseURL:         postgres.DatabaseURL,
		LogLevel:            "error",
		SessionCookieName:   testSessionCookieName,
		SessionTTL:          24 * time.Hour,
		SessionCookieSecure: false,
		MaxJSONBodyBytes:    2 << 20,
		StrictJSON:          true,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	t.Cleanup(app.Close)

	server := httptest.NewServer(app.Handler())
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	csrf := loginAndGetCSRFToken(t, client, server.URL)

	createReq := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/shopping-lists", `{"list_date":"2025-02-01","name":"Weekly Shop","notes":null}`)
	createReq.Header.Set("X-CSRF-Token", csrf)
	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("create shopping list: %v", err)
	}
	createBody := createResp.Body
	t.Cleanup(func() {
		if closeErr := createBody.Close(); closeErr != nil {
			t.Errorf("close create body: %v", closeErr)
		}
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status=%d, want %d", createResp.StatusCode, http.StatusCreated)
	}
	var created testShoppingListResponse
	if decodeErr := json.NewDecoder(createBody).Decode(&created); decodeErr != nil {
		t.Fatalf("decode create: %v", decodeErr)
	}
	if created.Name != "Weekly Shop" {
		t.Fatalf("create name=%q, want Weekly Shop", created.Name)
	}
	if created.ListDate != "2025-02-01" {
		t.Fatalf("create list_date=%q, want 2025-02-01", created.ListDate)
	}

	listResp, err := client.Get(server.URL + "/api/v1/shopping-lists?start=2025-02-01&end=2025-02-28")
	if err != nil {
		t.Fatalf("list shopping lists: %v", err)
	}
	listBody := listResp.Body
	t.Cleanup(func() {
		if closeErr := listBody.Close(); closeErr != nil {
			t.Errorf("close list body: %v", closeErr)
		}
	})
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d, want %d", listResp.StatusCode, http.StatusOK)
	}
	var listed []testShoppingListResponse
	if decodeErr := json.NewDecoder(listBody).Decode(&listed); decodeErr != nil {
		t.Fatalf("decode list: %v", decodeErr)
	}
	if len(listed) != 1 {
		t.Fatalf("list count=%d, want 1", len(listed))
	}

	getResp, err := client.Get(server.URL + "/api/v1/shopping-lists/" + created.ID)
	if err != nil {
		t.Fatalf("get shopping list: %v", err)
	}
	getBody := getResp.Body
	t.Cleanup(func() {
		if closeErr := getBody.Close(); closeErr != nil {
			t.Errorf("close get body: %v", closeErr)
		}
	})
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status=%d, want %d", getResp.StatusCode, http.StatusOK)
	}
	var got testShoppingListDetailResponse
	if decodeErr := json.NewDecoder(getBody).Decode(&got); decodeErr != nil {
		t.Fatalf("decode get: %v", decodeErr)
	}
	if got.ID != created.ID {
		t.Fatalf("get id=%q, want %q", got.ID, created.ID)
	}

	updateReq := newJSONRequest(t, http.MethodPut, server.URL+"/api/v1/shopping-lists/"+created.ID, `{"list_date":"2025-02-02","name":"Updated Shop","notes":"Bring bags"}`)
	updateReq.Header.Set("X-CSRF-Token", csrf)
	updateResp, err := client.Do(updateReq)
	if err != nil {
		t.Fatalf("update shopping list: %v", err)
	}
	updateBody := updateResp.Body
	t.Cleanup(func() {
		if closeErr := updateBody.Close(); closeErr != nil {
			t.Errorf("close update body: %v", closeErr)
		}
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("update status=%d, want %d", updateResp.StatusCode, http.StatusOK)
	}
	var updated testShoppingListResponse
	if decodeErr := json.NewDecoder(updateBody).Decode(&updated); decodeErr != nil {
		t.Fatalf("decode update: %v", decodeErr)
	}
	if updated.Name != "Updated Shop" {
		t.Fatalf("update name=%q, want Updated Shop", updated.Name)
	}
	if updated.ListDate != "2025-02-02" {
		t.Fatalf("update list_date=%q, want 2025-02-02", updated.ListDate)
	}

	deleteReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/shopping-lists/"+created.ID, nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	deleteReq.Header.Set("X-CSRF-Token", csrf)
	deleteResp, err := client.Do(deleteReq)
	if err != nil {
		t.Fatalf("delete shopping list: %v", err)
	}
	deleteBody := deleteResp.Body
	t.Cleanup(func() {
		if closeErr := deleteBody.Close(); closeErr != nil {
			t.Errorf("close delete body: %v", closeErr)
		}
	})
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d, want %d", deleteResp.StatusCode, http.StatusNoContent)
	}
}

func TestShoppingListItems_Flow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)

	user, userErr := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	})
	if userErr != nil {
		t.Fatalf("bootstrap user: %v", userErr)
	}

	milkItem, err := queries.CreateItem(ctx, sqlc.CreateItemParams{
		Name:      "milk",
		StoreUrl:  pgtype.Text{},
		AisleID:   pgtype.UUID{},
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if err != nil {
		t.Fatalf("create item milk: %v", err)
	}
	pepperItem, err := queries.CreateItem(ctx, sqlc.CreateItemParams{
		Name:      "pepper",
		StoreUrl:  pgtype.Text{},
		AisleID:   pgtype.UUID{},
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if err != nil {
		t.Fatalf("create item pepper: %v", err)
	}
	breadItem, err := queries.CreateItem(ctx, sqlc.CreateItemParams{
		Name:      "bread",
		StoreUrl:  pgtype.Text{},
		AisleID:   pgtype.UUID{},
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if err != nil {
		t.Fatalf("create item bread: %v", err)
	}

	recipe1, err := queries.CreateRecipe(ctx, sqlc.CreateRecipeParams{
		Title:            "Milk Soup",
		Servings:         2,
		PrepTimeMinutes:  5,
		TotalTimeMinutes: 10,
		CreatedBy:        user.ID,
		UpdatedBy:        user.ID,
	})
	if err != nil {
		t.Fatalf("create recipe1: %v", err)
	}
	recipe2, err := queries.CreateRecipe(ctx, sqlc.CreateRecipeParams{
		Title:            "Pepper Milk",
		Servings:         2,
		PrepTimeMinutes:  5,
		TotalTimeMinutes: 10,
		CreatedBy:        user.ID,
		UpdatedBy:        user.ID,
	})
	if err != nil {
		t.Fatalf("create recipe2: %v", err)
	}
	recipe3, err := queries.CreateRecipe(ctx, sqlc.CreateRecipeParams{
		Title:            "Bread Toast",
		Servings:         1,
		PrepTimeMinutes:  2,
		TotalTimeMinutes: 5,
		CreatedBy:        user.ID,
		UpdatedBy:        user.ID,
	})
	if err != nil {
		t.Fatalf("create recipe3: %v", err)
	}

	if err = queries.CreateRecipeIngredient(ctx, sqlc.CreateRecipeIngredientParams{
		RecipeID:     recipe1.ID,
		Position:     1,
		Quantity:     mustNumeric(t, 0.5),
		QuantityText: pgtype.Text{String: "1/2", Valid: true},
		Unit:         pgtype.Text{String: "lb", Valid: true},
		ItemID:       milkItem.ID,
		Prep:         pgtype.Text{},
		Notes:        pgtype.Text{},
		OriginalText: pgtype.Text{String: "1/2 lb milk", Valid: true},
		CreatedBy:    user.ID,
		UpdatedBy:    user.ID,
	}); err != nil {
		t.Fatalf("create ingredient recipe1: %v", err)
	}
	if err = queries.CreateRecipeIngredient(ctx, sqlc.CreateRecipeIngredientParams{
		RecipeID:     recipe2.ID,
		Position:     1,
		Quantity:     mustNumeric(t, 0.5),
		QuantityText: pgtype.Text{String: "1/2", Valid: true},
		Unit:         pgtype.Text{String: "lb", Valid: true},
		ItemID:       milkItem.ID,
		Prep:         pgtype.Text{},
		Notes:        pgtype.Text{},
		OriginalText: pgtype.Text{String: "1/2 lb milk", Valid: true},
		CreatedBy:    user.ID,
		UpdatedBy:    user.ID,
	}); err != nil {
		t.Fatalf("create ingredient recipe2 milk: %v", err)
	}
	if err = queries.CreateRecipeIngredient(ctx, sqlc.CreateRecipeIngredientParams{
		RecipeID:     recipe2.ID,
		Position:     2,
		Quantity:     pgtype.Numeric{},
		QuantityText: pgtype.Text{String: "to taste", Valid: true},
		Unit:         pgtype.Text{},
		ItemID:       pepperItem.ID,
		Prep:         pgtype.Text{},
		Notes:        pgtype.Text{},
		OriginalText: pgtype.Text{String: "pepper to taste", Valid: true},
		CreatedBy:    user.ID,
		UpdatedBy:    user.ID,
	}); err != nil {
		t.Fatalf("create ingredient recipe2 pepper: %v", err)
	}
	if err = queries.CreateRecipeIngredient(ctx, sqlc.CreateRecipeIngredientParams{
		RecipeID:     recipe3.ID,
		Position:     1,
		Quantity:     mustNumeric(t, 1.0),
		QuantityText: pgtype.Text{String: "1", Valid: true},
		Unit:         pgtype.Text{String: "loaf", Valid: true},
		ItemID:       breadItem.ID,
		Prep:         pgtype.Text{},
		Notes:        pgtype.Text{},
		OriginalText: pgtype.Text{String: "1 loaf bread", Valid: true},
		CreatedBy:    user.ID,
		UpdatedBy:    user.ID,
	}); err != nil {
		t.Fatalf("create ingredient recipe3 bread: %v", err)
	}

	planDate := pgtype.Date{Time: time.Date(2025, 2, 10, 0, 0, 0, 0, time.UTC), Valid: true}
	if _, err = queries.CreateMealPlanEntry(ctx, sqlc.CreateMealPlanEntryParams{
		UserID:    user.ID,
		PlanDate:  planDate,
		RecipeID:  recipe3.ID,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	}); err != nil {
		t.Fatalf("create meal plan entry: %v", err)
	}

	app, err := httpapi.New(ctx, logging.New("error"), config.Config{
		DatabaseURL:         postgres.DatabaseURL,
		LogLevel:            "error",
		SessionCookieName:   testSessionCookieName,
		SessionTTL:          24 * time.Hour,
		SessionCookieSecure: false,
		MaxJSONBodyBytes:    2 << 20,
		StrictJSON:          true,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	t.Cleanup(app.Close)

	server := httptest.NewServer(app.Handler())
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	csrf := loginAndGetCSRFToken(t, client, server.URL)

	createReq := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/shopping-lists", `{"list_date":"2025-02-09","name":"Weekly Shop","notes":null}`)
	createReq.Header.Set("X-CSRF-Token", csrf)
	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("create shopping list: %v", err)
	}
	createBody := createResp.Body
	t.Cleanup(func() {
		if closeErr := createBody.Close(); closeErr != nil {
			t.Errorf("close create body: %v", closeErr)
		}
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status=%d, want %d", createResp.StatusCode, http.StatusCreated)
	}
	var created testShoppingListResponse
	if decodeErr := json.NewDecoder(createBody).Decode(&created); decodeErr != nil {
		t.Fatalf("decode create: %v", decodeErr)
	}

	addManualReq := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/shopping-lists/"+created.ID+"/items", `{"items":[{"item_id":"`+uuid.UUID(milkItem.ID.Bytes).String()+`","quantity":1,"unit":"lb"}]}`)
	addManualReq.Header.Set("X-CSRF-Token", csrf)
	addManualResp, err := client.Do(addManualReq)
	if err != nil {
		t.Fatalf("add manual items: %v", err)
	}
	addManualBody := addManualResp.Body
	t.Cleanup(func() {
		if closeErr := addManualBody.Close(); closeErr != nil {
			t.Errorf("close manual body: %v", closeErr)
		}
	})
	if addManualResp.StatusCode != http.StatusOK {
		t.Fatalf("add manual status=%d, want %d", addManualResp.StatusCode, http.StatusOK)
	}

	addRecipesReq := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/shopping-lists/"+created.ID+"/items/from-recipes", `{"recipe_ids":["`+uuid.UUID(recipe1.ID.Bytes).String()+`","`+uuid.UUID(recipe2.ID.Bytes).String()+`"]}`)
	addRecipesReq.Header.Set("X-CSRF-Token", csrf)
	addRecipesResp, err := client.Do(addRecipesReq)
	if err != nil {
		t.Fatalf("add recipes: %v", err)
	}
	addRecipesBody := addRecipesResp.Body
	t.Cleanup(func() {
		if closeErr := addRecipesBody.Close(); closeErr != nil {
			t.Errorf("close recipes body: %v", closeErr)
		}
	})
	if addRecipesResp.StatusCode != http.StatusOK {
		t.Fatalf("add recipes status=%d, want %d", addRecipesResp.StatusCode, http.StatusOK)
	}
	var afterRecipes []testShoppingListItem
	if decodeErr := json.NewDecoder(addRecipesBody).Decode(&afterRecipes); decodeErr != nil {
		t.Fatalf("decode add recipes: %v", decodeErr)
	}

	milkEntry := findListItem(afterRecipes, "milk")
	if milkEntry == nil || milkEntry.Quantity == nil {
		t.Fatalf("missing milk entry after recipes")
	}
	if math.Abs(*milkEntry.Quantity-2.0) > 0.001 {
		t.Fatalf("milk quantity=%.2f, want 2.0", *milkEntry.Quantity)
	}
	if milkEntry.Unit == nil || *milkEntry.Unit != "lb" {
		t.Fatalf("milk unit=%v, want lb", milkEntry.Unit)
	}

	pepperEntry := findListItem(afterRecipes, "pepper")
	if pepperEntry == nil {
		t.Fatalf("missing pepper entry after recipes")
	}
	if pepperEntry.Unit == nil || *pepperEntry.Unit != "to taste" {
		t.Fatalf("pepper unit=%v, want to taste", pepperEntry.Unit)
	}
	if pepperEntry.Quantity != nil {
		t.Fatalf("pepper quantity=%v, want nil", pepperEntry.Quantity)
	}

	addMealPlanReq := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/shopping-lists/"+created.ID+"/items/from-meal-plan", `{"date":"2025-02-10"}`)
	addMealPlanReq.Header.Set("X-CSRF-Token", csrf)
	addMealPlanResp, err := client.Do(addMealPlanReq)
	if err != nil {
		t.Fatalf("add meal plan: %v", err)
	}
	addMealPlanBody := addMealPlanResp.Body
	t.Cleanup(func() {
		if closeErr := addMealPlanBody.Close(); closeErr != nil {
			t.Errorf("close meal plan body: %v", closeErr)
		}
	})
	if addMealPlanResp.StatusCode != http.StatusOK {
		t.Fatalf("add meal plan status=%d, want %d", addMealPlanResp.StatusCode, http.StatusOK)
	}
	var afterMealPlan []testShoppingListItem
	if decodeErr := json.NewDecoder(addMealPlanBody).Decode(&afterMealPlan); decodeErr != nil {
		t.Fatalf("decode meal plan: %v", decodeErr)
	}
	breadEntry := findListItem(afterMealPlan, "bread")
	if breadEntry == nil {
		t.Fatalf("missing bread entry after meal plan")
	}

	purchaseReq := newJSONRequest(t, http.MethodPatch, server.URL+"/api/v1/shopping-lists/"+created.ID+"/items/"+breadEntry.ID, `{"is_purchased":true}`)
	purchaseReq.Header.Set("X-CSRF-Token", csrf)
	purchaseResp, err := client.Do(purchaseReq)
	if err != nil {
		t.Fatalf("purchase item: %v", err)
	}
	purchaseBody := purchaseResp.Body
	t.Cleanup(func() {
		if closeErr := purchaseBody.Close(); closeErr != nil {
			t.Errorf("close purchase body: %v", closeErr)
		}
	})
	if purchaseResp.StatusCode != http.StatusOK {
		t.Fatalf("purchase status=%d, want %d", purchaseResp.StatusCode, http.StatusOK)
	}
	var purchased testShoppingListItem
	if decodeErr := json.NewDecoder(purchaseBody).Decode(&purchased); decodeErr != nil {
		t.Fatalf("decode purchase: %v", decodeErr)
	}
	if !purchased.IsPurchased {
		t.Fatalf("purchased is_purchased=false, want true")
	}
	if purchased.PurchasedAt == nil {
		t.Fatalf("purchased_at=nil, want value")
	}

	deleteReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/shopping-lists/"+created.ID+"/items/"+breadEntry.ID, nil)
	if err != nil {
		t.Fatalf("new delete item request: %v", err)
	}
	deleteReq.Header.Set("X-CSRF-Token", csrf)
	deleteResp, err := client.Do(deleteReq)
	if err != nil {
		t.Fatalf("delete item: %v", err)
	}
	deleteBody := deleteResp.Body
	t.Cleanup(func() {
		if closeErr := deleteBody.Close(); closeErr != nil {
			t.Errorf("close delete body: %v", closeErr)
		}
	})
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete item status=%d, want %d", deleteResp.StatusCode, http.StatusNoContent)
	}

	listResp, err := client.Get(server.URL + "/api/v1/shopping-lists/" + created.ID + "/items")
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	listBody := listResp.Body
	t.Cleanup(func() {
		if closeErr := listBody.Close(); closeErr != nil {
			t.Errorf("close list items body: %v", closeErr)
		}
	})
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list items status=%d, want %d", listResp.StatusCode, http.StatusOK)
	}
	var finalItems []testShoppingListItem
	if decodeErr := json.NewDecoder(listBody).Decode(&finalItems); decodeErr != nil {
		t.Fatalf("decode list items: %v", decodeErr)
	}
	if findListItem(finalItems, "bread") != nil {
		t.Fatalf("bread still present after delete")
	}
}

func mustNumeric(t *testing.T, value float64) pgtype.Numeric {
	t.Helper()

	var n pgtype.Numeric
	if scanErr := n.Scan(value); scanErr != nil {
		t.Fatalf("scan numeric: %v", scanErr)
	}
	return n
}

func findListItem(items []testShoppingListItem, name string) *testShoppingListItem {
	for i := range items {
		if items[i].Item.Name == name {
			return &items[i]
		}
	}
	return nil
}
