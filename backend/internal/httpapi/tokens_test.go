package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/bootstrap"
	"github.com/saiaj/cooking_app/backend/internal/config"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi"
	"github.com/saiaj/cooking_app/backend/internal/logging"
	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

func TestTokens_CreateListRevoke(t *testing.T) {
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

	req := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/tokens", `{"name":"laptop-cli"}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post tokens: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
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

	token, ok := created["token"].(string)
	if !ok {
		t.Fatalf("token missing or not string: %v", created["token"])
	}
	if !strings.HasPrefix(token, "cooking_app_pat_") {
		t.Fatalf("token prefix mismatch: %q", token)
	}

	var listed []map[string]any

	// Protected endpoint should reject unauthenticated requests.
	resp, err = http.Get(server.URL + "/api/v1/tokens")
	if err != nil {
		t.Fatalf("get tokens unauth: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth list status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	// PAT auth should work and update last_used_at.
	patReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/tokens", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	patReq.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(patReq)
	if err != nil {
		t.Fatalf("pat list: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pat list status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	listed = nil
	if decodeErr := json.NewDecoder(resp.Body).Decode(&listed); decodeErr != nil {
		t.Fatalf("decode pat list: %v", decodeErr)
	}
	if len(listed) != 1 {
		t.Fatalf("pat listed len=%d, want 1", len(listed))
	}
	if listed[0]["last_used_at"] == nil {
		t.Fatalf("expected last_used_at to be set after PAT-authenticated request")
	}

	resp, err = client.Get(server.URL + "/api/v1/tokens")
	if err != nil {
		t.Fatalf("get tokens: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d, want %d", resp.StatusCode, http.StatusOK)
	}

	listed = nil
	if decodeErr := json.NewDecoder(resp.Body).Decode(&listed); decodeErr != nil {
		t.Fatalf("decode list: %v", decodeErr)
	}
	if len(listed) != 1 {
		t.Fatalf("listed len=%d, want 1", len(listed))
	}
	if _, tokenPresent := listed[0]["token"]; tokenPresent {
		t.Fatalf("list unexpectedly includes token secret")
	}

	id, ok := listed[0]["id"].(string)
	if !ok || id == "" {
		t.Fatalf("id missing or not string: %v", listed[0]["id"])
	}
	deleteReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/tokens/"+id, nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	deleteReq.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(deleteReq)
	if err != nil {
		t.Fatalf("delete token: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	resp, err = client.Get(server.URL + "/api/v1/tokens")
	if err != nil {
		t.Fatalf("get tokens after delete: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
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
	if len(listed) != 0 {
		t.Fatalf("listed len=%d, want 0", len(listed))
	}
}

func TestTokens_ExpiredPATIsUnauthorized(t *testing.T) {
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

	expired := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	req := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/tokens", `{"name":"expired","expires_at":"`+expired+`"}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post tokens: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
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

	token, ok := created["token"].(string)
	if !ok || token == "" {
		t.Fatalf("token missing")
	}

	patReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/tokens", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	patReq.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(patReq)
	if err != nil {
		t.Fatalf("pat list: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}
