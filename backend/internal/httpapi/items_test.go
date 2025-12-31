package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
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

type testGroceryAisleResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SortGroup    int    `json:"sort_group"`
	SortOrder    int    `json:"sort_order"`
	NumericValue *int   `json:"numeric_value"`
}

type testItemResponse struct {
	ID       string                    `json:"id"`
	Name     string                    `json:"name"`
	StoreURL *string                   `json:"store_url"`
	Aisle    *testGroceryAisleResponse `json:"aisle"`
}

func TestItems_CRUD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)
	user, err := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	})
	if err != nil {
		t.Fatalf("bootstrap user: %v", err)
	}

	aisle, err := queries.CreateGroceryAisle(ctx, sqlc.CreateGroceryAisleParams{
		Name:         "Bakery",
		SortGroup:    0,
		SortOrder:    1,
		NumericValue: pgtype.Int4{},
		CreatedBy:    user.ID,
		UpdatedBy:    user.ID,
	})
	if err != nil {
		t.Fatalf("create aisle: %v", err)
	}

	app, err := httpapi.New(ctx, logging.New("error"), config.Config{
		DatabaseURL:         postgres.DatabaseURL,
		LogLevel:            "error",
		SessionCookieName:   "cooking_app_session",
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

	resp, err := http.Get(server.URL + "/api/v1/items")
	if err != nil {
		t.Fatalf("get items unauth: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth list status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("close unauth body: %v", closeErr)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	csrf := loginAndGetCSRFToken(t, client, server.URL)

	resp, err = client.Get(server.URL + "/api/v1/items")
	if err != nil {
		t.Fatalf("get items: %v", err)
	}
	listBody := resp.Body
	t.Cleanup(func() {
		if closeErr := listBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var listed []testItemResponse
	if decodeErr := json.NewDecoder(listBody).Decode(&listed); decodeErr != nil {
		t.Fatalf("decode list: %v", decodeErr)
	}
	if len(listed) != 0 {
		t.Fatalf("listed len=%d, want 0", len(listed))
	}

	aisleID := uuid.UUID(aisle.ID.Bytes).String()
	createBody := fmt.Sprintf(`{"name":"Milk","store_url":"https://example.com/milk","aisle_id":%q}`, aisleID)
	req := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/items", createBody)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post items: %v", err)
	}
	createRespBody := resp.Body
	t.Cleanup(func() {
		if closeErr := createRespBody.Close(); closeErr != nil {
			t.Errorf("close create body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var created testItemResponse
	if decodeErr := json.NewDecoder(createRespBody).Decode(&created); decodeErr != nil {
		t.Fatalf("decode create: %v", decodeErr)
	}
	if created.ID == "" || created.Name != "Milk" {
		t.Fatalf("created=%#v, want id and name", created)
	}
	if created.StoreURL == nil || *created.StoreURL != "https://example.com/milk" {
		t.Fatalf("store_url=%v, want set", created.StoreURL)
	}
	if created.Aisle == nil || created.Aisle.Name != "Bakery" {
		t.Fatalf("aisle=%v, want bakery", created.Aisle)
	}

	req = newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/items", `{"name":"milk"}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post items dup: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("dup status=%d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("close dup body: %v", closeErr)
	}

	updateBody := `{"name":"Whole Milk","store_url":null,"aisle_id":null}`
	req = newJSONRequest(t, http.MethodPut, server.URL+"/api/v1/items/"+created.ID, updateBody)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("update item: %v", err)
	}
	updateRespBody := resp.Body
	t.Cleanup(func() {
		if closeErr := updateRespBody.Close(); closeErr != nil {
			t.Errorf("close update body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var updated testItemResponse
	if decodeErr := json.NewDecoder(updateRespBody).Decode(&updated); decodeErr != nil {
		t.Fatalf("decode update: %v", decodeErr)
	}
	if updated.Name != "Whole Milk" || updated.StoreURL != nil || updated.Aisle != nil {
		t.Fatalf("updated=%#v, want cleared fields", updated)
	}

	resp, err = client.Get(server.URL + "/api/v1/items/" + created.ID)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	getBody := resp.Body
	t.Cleanup(func() {
		if closeErr := getBody.Close(); closeErr != nil {
			t.Errorf("close get body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got testItemResponse
	if decodeErr := json.NewDecoder(getBody).Decode(&got); decodeErr != nil {
		t.Fatalf("decode get: %v", decodeErr)
	}
	if got.ID != created.ID || got.Name != "Whole Milk" {
		t.Fatalf("got=%#v, want updated values", got)
	}

	req, err = http.NewRequest(http.MethodDelete, server.URL+"/api/v1/items/"+created.ID, nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("delete item: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("close delete body: %v", closeErr)
	}
}
