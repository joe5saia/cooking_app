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

func TestRateLimit_Login(t *testing.T) {
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
		DatabaseURL:                postgres.DatabaseURL,
		LogLevel:                   "error",
		SessionCookieName:          testSessionCookieName,
		SessionTTL:                 24 * time.Hour,
		SessionCookieSecure:        false,
		MaxJSONBodyBytes:           2 << 20,
		StrictJSON:                 true,
		LoginRateLimitPerMin:       1,
		LoginRateLimitBurst:        1,
		TokenCreateRateLimitPerMin: 0,
		TokenCreateRateLimitBurst:  0,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	t.Cleanup(app.Close)

	server := httptest.NewServer(app.Handler())
	t.Cleanup(server.Close)

	resp, err := http.Post(server.URL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"username":"joe","password":"pw"}`))
	if err != nil {
		t.Fatalf("post login: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	resp, err = http.Post(server.URL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"username":"joe","password":"pw"}`))
	if err != nil {
		t.Fatalf("post login: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	problem := decodeProblem(t, resp.Body)
	if problem.Code != "rate_limited" {
		t.Fatalf("code=%q, want %q", problem.Code, "rate_limited")
	}
}

func TestRateLimit_TokenCreate(t *testing.T) {
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
		DatabaseURL:                postgres.DatabaseURL,
		LogLevel:                   "error",
		SessionCookieName:          testSessionCookieName,
		SessionTTL:                 24 * time.Hour,
		SessionCookieSecure:        false,
		MaxJSONBodyBytes:           2 << 20,
		StrictJSON:                 true,
		LoginRateLimitPerMin:       0,
		LoginRateLimitBurst:        0,
		TokenCreateRateLimitPerMin: 1,
		TokenCreateRateLimitBurst:  1,
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

	req := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/tokens", `{"name":"cli"}`)
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
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusOK)
	}

	var created map[string]any
	if decodeErr := json.NewDecoder(resp.Body).Decode(&created); decodeErr != nil {
		t.Fatalf("decode create: %v", decodeErr)
	}
	if token, ok := created["token"].(string); !ok || token == "" {
		t.Fatalf("token missing")
	}

	req = newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/tokens", `{"name":"cli-2"}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post tokens: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	problem := decodeProblem(t, resp.Body)
	if problem.Code != "rate_limited" {
		t.Fatalf("code=%q, want %q", problem.Code, "rate_limited")
	}
}
