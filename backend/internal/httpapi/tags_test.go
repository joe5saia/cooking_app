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

func TestTags_CRUD(t *testing.T) {
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

	resp, err := http.Get(server.URL + "/api/v1/tags")
	if err != nil {
		t.Fatalf("get tags unauth: %v", err)
	}
	body := resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth list status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	csrf := loginAndGetCSRFToken(t, client, server.URL)

	resp, err = client.Get(server.URL + "/api/v1/tags")
	if err != nil {
		t.Fatalf("get tags: %v", err)
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
	var listed []map[string]any
	if decodeErr := json.NewDecoder(listBody).Decode(&listed); decodeErr != nil {
		t.Fatalf("decode list: %v", decodeErr)
	}
	if len(listed) != 0 {
		t.Fatalf("listed len=%d, want 0", len(listed))
	}

	req := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/tags", `{"name":"Soup"}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post tags: %v", err)
	}
	createBody := resp.Body
	t.Cleanup(func() {
		if closeErr := createBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var created map[string]any
	if decodeErr := json.NewDecoder(createBody).Decode(&created); decodeErr != nil {
		t.Fatalf("decode create: %v", decodeErr)
	}
	id, ok := created["id"].(string)
	if !ok || id == "" {
		t.Fatalf("id missing or not string: %v", created["id"])
	}
	if created["name"] != "Soup" {
		t.Fatalf("name=%v, want Soup", created["name"])
	}

	req = newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/tags", `{"name":"soup"}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post tags dup: %v", err)
	}
	dupBody := resp.Body
	t.Cleanup(func() {
		if closeErr := dupBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("dup status=%d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	req = newJSONRequest(t, http.MethodPut, server.URL+"/api/v1/tags/"+id, `{"name":"Dinner"}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("update tag: %v", err)
	}
	updateBody := resp.Body
	t.Cleanup(func() {
		if closeErr := updateBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var updated map[string]any
	if decodeErr := json.NewDecoder(updateBody).Decode(&updated); decodeErr != nil {
		t.Fatalf("decode update: %v", decodeErr)
	}
	if updated["name"] != "Dinner" {
		t.Fatalf("name=%v, want Dinner", updated["name"])
	}

	req, err = http.NewRequest(http.MethodDelete, server.URL+"/api/v1/tags/"+id, nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	deleteBody := resp.Body
	t.Cleanup(func() {
		if closeErr := deleteBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}
