package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
	"github.com/saiaj/cooking_app/backend/internal/logging"
	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

func decodeProblem(t *testing.T, body io.Reader) response.Problem {
	t.Helper()

	var p response.Problem
	if err := json.NewDecoder(body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	return p
}

const problemCodeUnauthorized = "unauthorized"

func TestAuth_LoginSetsCookieAttributes(t *testing.T) {
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
		SessionCookieSecure: true,
		MaxJSONBodyBytes:    2 << 20,
		StrictJSON:          true,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	t.Cleanup(app.Close)

	server := httptest.NewServer(app.Handler())
	t.Cleanup(server.Close)

	body, err := json.Marshal(map[string]string{"username": "joe", "password": "pw"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(server.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
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

	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatalf("expected Set-Cookie")
	}
	got := strings.Join(cookies, "\n")
	if !strings.Contains(got, "cooking_app_session=") {
		t.Fatalf("missing cookie name: %q", got)
	}
	if !strings.Contains(got, "HttpOnly") {
		t.Fatalf("missing HttpOnly: %q", got)
	}
	if !strings.Contains(got, "SameSite=Lax") {
		t.Fatalf("missing SameSite=Lax: %q", got)
	}
	if !strings.Contains(got, "Path=/") {
		t.Fatalf("missing Path=/: %q", got)
	}
	if !strings.Contains(got, "Secure") {
		t.Fatalf("missing Secure: %q", got)
	}
}

func TestAuth_LoginLogoutMeFlow(t *testing.T) {
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

	resp, err := client.Get(server.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatalf("get me: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me status=%d, want %d", resp.StatusCode, http.StatusOK)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/logout", nil)
	if err != nil {
		t.Fatalf("new logout request: %v", err)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("post logout: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	resp, err = client.Get(server.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatalf("get me after logout: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me-after-logout status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	problem := decodeProblem(t, resp.Body)
	if problem.Code != problemCodeUnauthorized {
		t.Fatalf("code=%q, want %q", problem.Code, problemCodeUnauthorized)
	}
	if problem.Message == "" {
		t.Fatalf("expected non-empty message")
	}
}

func TestAuth_MeRequiresAuth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

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

	resp, err := http.Get(server.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatalf("get me: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	problem := decodeProblem(t, resp.Body)
	if problem.Code != problemCodeUnauthorized {
		t.Fatalf("code=%q, want %q", problem.Code, problemCodeUnauthorized)
	}
}

func TestAuth_LogoutRequiresAuth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

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

	resp, err := http.Post(server.URL+"/api/v1/auth/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("post logout: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	problem := decodeProblem(t, resp.Body)
	if problem.Code != problemCodeUnauthorized {
		t.Fatalf("code=%q, want %q", problem.Code, problemCodeUnauthorized)
	}
}
