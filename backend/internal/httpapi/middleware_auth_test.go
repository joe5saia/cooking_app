package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/auth/pat"
	"github.com/saiaj/cooking_app/backend/internal/bootstrap"
	"github.com/saiaj/cooking_app/backend/internal/config"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/logging"
	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

func TestParseBearerToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		header      string
		wantToken   string
		wantPresent bool
	}{
		{name: "empty", header: "", wantToken: "", wantPresent: false},
		{name: "wrong kind", header: "Basic abc", wantToken: "", wantPresent: false},
		{name: "missing space", header: "Bearerabc", wantToken: "", wantPresent: false},
		{name: "missing value", header: "Bearer ", wantToken: "", wantPresent: false},
		{name: "ok", header: "Bearer abc", wantToken: "abc", wantPresent: true},
		{name: "case insensitive", header: "bearer abc", wantToken: "abc", wantPresent: true},
		{name: "trims", header: "Bearer   abc  ", wantToken: "abc", wantPresent: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotToken, gotPresent := parseBearerToken(tt.header)
			if gotPresent != tt.wantPresent {
				t.Fatalf("present=%v, want %v", gotPresent, tt.wantPresent)
			}
			if gotToken != tt.wantToken {
				t.Fatalf("token=%q, want %q", gotToken, tt.wantToken)
			}
		})
	}
}

func TestAuthMiddleware_NoAuth_Unauthorized(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	app, err := New(ctx, logging.New("error"), config.Config{
		DatabaseURL:         postgres.DatabaseURL,
		LogLevel:            "error",
		SessionCookieName:   "cooking_app_session",
		SessionTTL:          24 * time.Hour,
		SessionCookieSecure: false,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	t.Cleanup(app.Close)

	nextCalled := false
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.test/protected", nil)
	handler.ServeHTTP(rr, req)

	if nextCalled {
		t.Fatalf("expected next handler not to be called")
	}
	if rr.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want %d", rr.Result().StatusCode, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_SessionAuth_SetsContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	app, err := New(ctx, logging.New("error"), config.Config{
		DatabaseURL:         postgres.DatabaseURL,
		LogLevel:            "error",
		SessionCookieName:   "cooking_app_session",
		SessionTTL:          24 * time.Hour,
		SessionCookieSecure: false,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	t.Cleanup(app.Close)

	user, err := bootstrap.CreateFirstUser(ctx, app.queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	})
	if err != nil {
		t.Fatalf("bootstrap user: %v", err)
	}

	sessionToken, sessionHash, err := newSessionToken()
	if err != nil {
		t.Fatalf("new session token: %v", err)
	}
	if err := app.createSession(ctx, user.ID, sessionHash, time.Now().Add(2*time.Hour).UTC()); err != nil {
		t.Fatalf("create session: %v", err)
	}

	var got authInfo
	nextCalled := false
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		info, ok := authInfoFromRequest(r)
		if !ok {
			t.Fatalf("expected auth info in context")
		}
		got = info
		w.WriteHeader(http.StatusNoContent)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.test/protected", nil)
	req.AddCookie(&http.Cookie{Name: app.sessionCookieName, Value: sessionToken})
	handler.ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatalf("expected next handler to be called")
	}
	if rr.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d, want %d", rr.Result().StatusCode, http.StatusNoContent)
	}
	if got.AuthType != authTypeSession {
		t.Fatalf("auth_type=%q, want %q", got.AuthType, authTypeSession)
	}
	if got.UserID.String() != uuidString(user.ID) {
		t.Fatalf("user_id=%s, want %s", got.UserID.String(), uuidString(user.ID))
	}
}

func TestAuthMiddleware_PATAuth_SetsContextAndTouchesLastUsed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	app, err := New(ctx, logging.New("error"), config.Config{
		DatabaseURL:         postgres.DatabaseURL,
		LogLevel:            "error",
		SessionCookieName:   "cooking_app_session",
		SessionTTL:          24 * time.Hour,
		SessionCookieSecure: false,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	t.Cleanup(app.Close)

	user, err := bootstrap.CreateFirstUser(ctx, app.queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	})
	if err != nil {
		t.Fatalf("bootstrap user: %v", err)
	}

	secret, hash, err := pat.Generate()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}

	created, err := app.queries.CreateToken(ctx, sqlc.CreateTokenParams{
		UserID:    user.ID,
		Name:      "cli",
		TokenHash: hash,
		LastUsedAt: pgtype.Timestamptz{
			Valid: false,
		},
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(2 * time.Hour).UTC(),
			Valid: true,
		},
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	var got authInfo
	nextCalled := false
	handler := app.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		info, ok := authInfoFromRequest(r)
		if !ok {
			t.Fatalf("expected auth info in context")
		}
		got = info
		w.WriteHeader(http.StatusNoContent)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.test/protected", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	handler.ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatalf("expected next handler to be called")
	}
	if rr.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d, want %d", rr.Result().StatusCode, http.StatusNoContent)
	}
	if got.AuthType != authTypePAT {
		t.Fatalf("auth_type=%q, want %q", got.AuthType, authTypePAT)
	}
	if got.UserID.String() != uuidString(user.ID) {
		t.Fatalf("user_id=%s, want %s", got.UserID.String(), uuidString(user.ID))
	}

	tokens, err := app.queries.ListTokensByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}

	var touched *sqlc.PersonalAccessToken
	for i := range tokens {
		if tokens[i].ID.Valid && created.ID.Valid && tokens[i].ID.Bytes == created.ID.Bytes {
			touched = &tokens[i]
			break
		}
	}
	if touched == nil {
		t.Fatalf("created token not found in list")
	}
	if !touched.LastUsedAt.Valid {
		t.Fatalf("expected last_used_at to be set")
	}
}
