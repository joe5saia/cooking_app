package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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
	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Key = "ts"
			}
			return a
		},
	}
	return slog.New(slog.NewJSONHandler(buf, opts))
}

func decodeJSONLines(t *testing.T, s string) []map[string]any {
	t.Helper()

	lines := strings.Split(s, "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("unmarshal log line: %v\nline=%s", err, line)
		}
		out = append(out, m)
	}
	return out
}

func TestRequestLogger_LogsBasics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	var buf bytes.Buffer
	logger := newTestLogger(&buf)

	app, err := httpapi.New(ctx, logger, config.Config{
		DatabaseURL:         postgres.DatabaseURL,
		LogLevel:            "info",
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

	resp, err := http.Get(server.URL + "/api/v1/healthz?x=1")
	if err != nil {
		t.Fatalf("get healthz: %v", err)
	}
	body := resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusOK)
	}

	events := decodeJSONLines(t, buf.String())
	found := false
	for _, evt := range events {
		if evt["msg"] != "request" {
			continue
		}
		if evt["path"] != "/api/v1/healthz" {
			continue
		}
		found = true

		if evt["ts"] == nil {
			t.Fatalf("missing ts")
		}
		if evt["level"] == nil {
			t.Fatalf("missing level")
		}
		if evt["request_id"] == "" {
			t.Fatalf("missing request_id")
		}
		if evt["remote_ip"] == "" {
			t.Fatalf("missing remote_ip")
		}
		if evt["method"] != http.MethodGet {
			t.Fatalf("method=%v, want %s", evt["method"], http.MethodGet)
		}
		if status, ok := evt["status"].(float64); !ok || int(status) != http.StatusOK {
			t.Fatalf("status=%v, want %d", evt["status"], http.StatusOK)
		}
		if _, ok := evt["duration_ms"].(float64); !ok {
			t.Fatalf("duration_ms=%T, want number", evt["duration_ms"])
		}
		if _, ok := evt["auth_type"]; ok {
			t.Fatalf("unexpected auth_type on public endpoint")
		}
	}
	if !found {
		t.Fatalf("request log not found")
	}
}

func TestRequestLogger_LogsAuthTypeForPAT(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)
	_, err := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	})
	if err != nil {
		t.Fatalf("bootstrap user: %v", err)
	}

	var buf bytes.Buffer
	logger := newTestLogger(&buf)

	app, err := httpapi.New(ctx, logger, config.Config{
		DatabaseURL:         postgres.DatabaseURL,
		LogLevel:            "info",
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

	req := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/tokens", `{"name":"cli"}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post tokens: %v", err)
	}
	createBody := resp.Body
	t.Cleanup(func() {
		if closeErr := createBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create token status=%d, want %d", resp.StatusCode, http.StatusOK)
	}

	var created map[string]any
	if decodeErr := json.NewDecoder(resp.Body).Decode(&created); decodeErr != nil {
		t.Fatalf("decode create token: %v", decodeErr)
	}
	secret, ok := created["token"].(string)
	if !ok || secret == "" {
		t.Fatalf("token missing")
	}

	listReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/tokens", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	listReq.Header.Set("Authorization", "Bearer "+secret)
	resp, err = http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatalf("pat list: %v", err)
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

	if strings.Contains(buf.String(), secret) {
		t.Fatalf("log output contains PAT secret")
	}

	events := decodeJSONLines(t, buf.String())
	found := false
	for _, evt := range events {
		if evt["msg"] != "request" {
			continue
		}
		if evt["path"] != "/api/v1/tokens" {
			continue
		}
		if evt["auth_type"] != "pat" {
			continue
		}
		userID, ok := evt["user_id"].(string)
		if !ok || userID == "" {
			t.Fatalf("missing user_id")
		}
		found = true
		break
	}
	if !found {
		t.Fatalf("request log with auth_type=pat not found")
	}
}
