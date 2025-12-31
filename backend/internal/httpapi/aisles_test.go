package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/bootstrap"
	"github.com/saiaj/cooking_app/backend/internal/config"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi"
	"github.com/saiaj/cooking_app/backend/internal/logging"
	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

type testAisleResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SortGroup    int    `json:"sort_group"`
	SortOrder    int    `json:"sort_order"`
	NumericValue *int   `json:"numeric_value"`
}

func TestAisles_CRUD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)
	if _, err := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	}); err != nil {
		t.Fatalf("bootstrap user: %v", err)
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

	resp, err := http.Get(server.URL + "/api/v1/aisles")
	if err != nil {
		t.Fatalf("get aisles unauth: %v", err)
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

	resp, err = client.Get(server.URL + "/api/v1/aisles")
	if err != nil {
		t.Fatalf("get aisles: %v", err)
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
	var listed []testAisleResponse
	if decodeErr := json.NewDecoder(listBody).Decode(&listed); decodeErr != nil {
		t.Fatalf("decode list: %v", decodeErr)
	}
	if len(listed) != 0 {
		t.Fatalf("listed len=%d, want 0", len(listed))
	}

	req := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/aisles", `{"name":"Bakery","sort_group":0,"sort_order":1}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post aisles: %v", err)
	}
	createBody := resp.Body
	t.Cleanup(func() {
		if closeErr := createBody.Close(); closeErr != nil {
			t.Errorf("close create body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var created testAisleResponse
	if decodeErr := json.NewDecoder(createBody).Decode(&created); decodeErr != nil {
		t.Fatalf("decode create: %v", decodeErr)
	}
	if created.ID == "" || created.Name != "Bakery" {
		t.Fatalf("created=%#v, want id and name", created)
	}

	req = newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/aisles", `{"name":"bakery","sort_group":0,"sort_order":2}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post aisles dup: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("dup status=%d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("close dup body: %v", closeErr)
	}

	req = newJSONRequest(t, http.MethodPut, server.URL+"/api/v1/aisles/"+created.ID, `{"name":"Front","sort_group":0,"sort_order":0}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("update aisle: %v", err)
	}
	updateBody := resp.Body
	t.Cleanup(func() {
		if closeErr := updateBody.Close(); closeErr != nil {
			t.Errorf("close update body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var updated testAisleResponse
	if decodeErr := json.NewDecoder(updateBody).Decode(&updated); decodeErr != nil {
		t.Fatalf("decode update: %v", decodeErr)
	}
	if updated.Name != "Front" || updated.SortOrder != 0 {
		t.Fatalf("updated=%#v, want updated values", updated)
	}

	resp, err = client.Get(server.URL + "/api/v1/aisles/" + created.ID)
	if err != nil {
		t.Fatalf("get aisle: %v", err)
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
	var got testAisleResponse
	if decodeErr := json.NewDecoder(getBody).Decode(&got); decodeErr != nil {
		t.Fatalf("decode get: %v", decodeErr)
	}
	if got.ID != created.ID || got.Name != "Front" {
		t.Fatalf("got=%#v, want updated aisle", got)
	}

	req, err = http.NewRequest(http.MethodDelete, server.URL+"/api/v1/aisles/"+created.ID, nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("delete aisle: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("close delete body: %v", closeErr)
	}
}

func TestAisles_Validation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)
	if _, err := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	}); err != nil {
		t.Fatalf("bootstrap user: %v", err)
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

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	csrf := loginAndGetCSRFToken(t, client, server.URL)

	req := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/aisles", `{"name":"Bad","sort_group":4,"sort_order":1}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post aisles invalid: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid status=%d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("close invalid body: %v", closeErr)
	}
}
