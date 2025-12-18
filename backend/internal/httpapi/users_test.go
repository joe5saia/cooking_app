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

func TestUsers_ListCreateDeactivate(t *testing.T) {
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

	resp, err := http.Get(server.URL + "/api/v1/users")
	if err != nil {
		t.Fatalf("get users unauth: %v", err)
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

	resp, err = client.Get(server.URL + "/api/v1/users")
	if err != nil {
		t.Fatalf("get users: %v", err)
	}
	body = resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d, want %d", resp.StatusCode, http.StatusOK)
	}

	var listed []map[string]any
	if decodeErr := json.NewDecoder(resp.Body).Decode(&listed); decodeErr != nil {
		t.Fatalf("decode list: %v", decodeErr)
	}
	if len(listed) != 1 {
		t.Fatalf("listed len=%d, want 1", len(listed))
	}
	if listed[0]["username"] != "joe" {
		t.Fatalf("username=%v, want joe", listed[0]["username"])
	}
	if _, passwordPresent := listed[0]["password_hash"]; passwordPresent {
		t.Fatalf("list unexpectedly includes password_hash")
	}

	req := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/users", `{"username":"shannon","password":"pw2","display_name":"Shannon"}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post users: %v", err)
	}
	body = resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status=%d, want %d", resp.StatusCode, http.StatusOK)
	}

	var created map[string]any
	if decodeErr := json.NewDecoder(resp.Body).Decode(&created); decodeErr != nil {
		t.Fatalf("decode create: %v", decodeErr)
	}
	if _, passwordPresent := created["password_hash"]; passwordPresent {
		t.Fatalf("create unexpectedly includes password_hash")
	}
	id, ok := created["id"].(string)
	if !ok || id == "" {
		t.Fatalf("id missing or not string: %v", created["id"])
	}

	resp, err = client.Get(server.URL + "/api/v1/users")
	if err != nil {
		t.Fatalf("get users 2: %v", err)
	}
	body = resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list2 status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	listed = nil
	if decodeErr := json.NewDecoder(resp.Body).Decode(&listed); decodeErr != nil {
		t.Fatalf("decode list2: %v", decodeErr)
	}
	if len(listed) != 2 {
		t.Fatalf("listed len=%d, want 2", len(listed))
	}

	deactivateReq, err := http.NewRequest(http.MethodPut, server.URL+"/api/v1/users/"+id+"/deactivate", nil)
	if err != nil {
		t.Fatalf("new deactivate request: %v", err)
	}
	deactivateReq.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(deactivateReq)
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	body = resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("deactivate status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	resp, err = client.Get(server.URL + "/api/v1/users")
	if err != nil {
		t.Fatalf("get users 3: %v", err)
	}
	body = resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list3 status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	listed = nil
	if decodeErr := json.NewDecoder(resp.Body).Decode(&listed); decodeErr != nil {
		t.Fatalf("decode list3: %v", decodeErr)
	}

	found := false
	for _, u := range listed {
		if u["id"] == id {
			found = true
			if isActive, ok := u["is_active"].(bool); !ok || isActive {
				t.Fatalf("is_active=%v, want false", u["is_active"])
			}
		}
	}
	if !found {
		t.Fatalf("deactivated user not found in list")
	}

	req = newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/users", `{"username":"","password":"pw2"}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post users validation: %v", err)
	}
	body = resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("validation status=%d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
