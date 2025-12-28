package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/config"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/credentials"
)

func TestRunAuthLoginStoresToken(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "cooking_app_session", Value: "sess", Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "cooking_app_session_csrf", Value: "csrf123", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-CSRF-Token"); got != "csrf123" {
			t.Fatalf("csrf header = %q, want %q", got, "csrf123")
		}
		resp := struct {
			ID        string    `json:"id"`
			Name      string    `json:"name"`
			Token     string    `json:"token"`
			CreatedAt time.Time `json:"created_at"`
		}{
			ID:        "token-1",
			Name:      "cookctl",
			Token:     "pat_abc",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	mux.HandleFunc("/api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString("pw"),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runAuthLogin([]string{"--username", "sam", "--password-stdin"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	creds, ok, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected credentials to be stored")
	}
	if creds.Token != "pat_abc" {
		t.Fatalf("Token = %q, want %q", creds.Token, "pat_abc")
	}
	if creds.TokenID != testTokenID {
		t.Fatalf("TokenID = %q, want %q", creds.TokenID, testTokenID)
	}
	if creds.APIURL != server.URL {
		t.Fatalf("APIURL = %q, want %q", creds.APIURL, server.URL)
	}
}

func TestRunAuthLogoutRevokeSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tokens/token-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer pat_abc" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer pat_abc")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{
		Token:   "pat_abc",
		TokenID: "token-1",
		APIURL:  server.URL,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runAuthLogout([]string{"--revoke"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	_, ok, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected credentials to be cleared")
	}
}

func TestRunAuthLogoutRevokeMissingID(t *testing.T) {
	t.Parallel()

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg:    config.Config{},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runAuthLogout([]string{"--revoke"})
	if exitCode != exitError {
		t.Fatalf("exit code = %d, want %d", exitCode, exitError)
	}
}

func TestRunAuthStatusUsesStoredAPIURL(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(storePath)
	createdAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	expiresAt := time.Date(2025, 2, 2, 3, 4, 5, 0, time.UTC)
	if err := store.Save(credentials.Credentials{
		Token:     "pat_123",
		TokenID:   testTokenID,
		TokenName: "cli",
		CreatedAt: &createdAt,
		ExpiresAt: &expiresAt,
		APIURL:    "http://stored.local",
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
			APIURL:  "http://config.local",
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runAuthStatus(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var status authStatus
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&status); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if status.Source != string(tokenSourceCredentials) {
		t.Fatalf("source = %q, want %q", status.Source, tokenSourceCredentials)
	}
	if status.APIURL != "http://stored.local" {
		t.Fatalf("api url = %q, want %q", status.APIURL, "http://stored.local")
	}
	if status.TokenID != testTokenID {
		t.Fatalf("token id = %q, want %q", status.TokenID, testTokenID)
	}
	if status.TokenName != "cli" {
		t.Fatalf("token name = %q, want %q", status.TokenName, "cli")
	}
	if status.CreatedAt == nil || !status.CreatedAt.Equal(createdAt) {
		t.Fatalf("created_at = %v, want %v", status.CreatedAt, createdAt)
	}
	if status.ExpiresAt == nil || !status.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expires_at = %v, want %v", status.ExpiresAt, expiresAt)
	}
	if !status.TokenPresent {
		t.Fatalf("expected token present")
	}
}

func TestRunAuthStatusUsesOverrideAPIURL(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(storePath)
	if err := store.Save(credentials.Credentials{
		Token:  "pat_123",
		APIURL: "http://stored.local",
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
			APIURL:  "http://override.local",
		},
		stdin:          bytes.NewBufferString(""),
		stdout:         stdout,
		stderr:         &bytes.Buffer{},
		store:          store,
		apiURLOverride: true,
	}

	exitCode := app.runAuthStatus(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var status authStatus
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&status); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if status.APIURL != "http://override.local" {
		t.Fatalf("api url = %q, want %q", status.APIURL, "http://override.local")
	}
	if status.Source != string(tokenSourceCredentials) {
		t.Fatalf("source = %q, want %q", status.Source, tokenSourceCredentials)
	}
}

func TestRunAuthStatusUsesConfigAPIURLForEnvToken(t *testing.T) {
	t.Setenv("COOKING_PAT", "pat_env")

	storePath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(storePath)
	if err := store.Save(credentials.Credentials{
		Token:  "pat_123",
		APIURL: "http://stored.local",
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
			APIURL:  "http://config.local",
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runAuthStatus(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var status authStatus
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&status); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if status.Source != string(tokenSourceEnv) {
		t.Fatalf("source = %q, want %q", status.Source, tokenSourceEnv)
	}
	if status.APIURL != "http://config.local" {
		t.Fatalf("api url = %q, want %q", status.APIURL, "http://config.local")
	}
	if !status.TokenPresent {
		t.Fatalf("expected token present")
	}
}

func TestRunAuthSetWithAPIURL(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "credentials.json")
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
			APIURL:  "http://default.local",
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(storePath),
	}

	exitCode := app.runAuthSet([]string{"--token", "pat_123", "--api-url", "http://custom.local"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	creds, ok, err := app.store.Load()
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if !ok {
		t.Fatalf("expected stored credentials")
	}
	if creds.APIURL != "http://custom.local" {
		t.Fatalf("api url = %q, want %q", creds.APIURL, "http://custom.local")
	}
	if creds.Token != "pat_123" {
		t.Fatalf("token = %q, want %q", creds.Token, "pat_123")
	}
}

func TestRunAuthSetTokenStdin(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "credentials.json")
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
			APIURL:  "http://default.local",
		},
		stdin:  bytes.NewBufferString("pat_stdin\n"),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(storePath),
	}

	exitCode := app.runAuthSet([]string{"--token-stdin"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	creds, ok, err := app.store.Load()
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if !ok {
		t.Fatalf("expected stored credentials")
	}
	if creds.APIURL != "http://default.local" {
		t.Fatalf("api url = %q, want %q", creds.APIURL, "http://default.local")
	}
	if creds.Token != "pat_stdin" {
		t.Fatalf("token = %q, want %q", creds.Token, "pat_stdin")
	}
}

func TestRunAuthSetTokenStdinConflict(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "credentials.json")
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
			APIURL:  "http://default.local",
		},
		stdin:  bytes.NewBufferString("pat_conflict"),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(storePath),
	}

	exitCode := app.runAuthSet([]string{"--token", "pat_123", "--token-stdin"})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}

	if _, ok, err := app.store.Load(); err != nil {
		t.Fatalf("load credentials: %v", err)
	} else if ok {
		t.Fatalf("expected no stored credentials")
	}
}
