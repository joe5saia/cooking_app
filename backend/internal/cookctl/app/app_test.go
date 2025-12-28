package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/config"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/credentials"
)

// writeTestJSON writes a JSON response and fails the test on error.
func writeTestJSON(t *testing.T, w http.ResponseWriter, payload interface{}) {
	t.Helper()
	if err := writeJSON(w, payload); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

const (
	testVersion          = "v1.2.3"
	testCommit           = "abc123"
	testBuiltAt          = "2025-01-02T03:04:05Z"
	testTokenID          = "token-1"
	testMealPlanDate     = "2025-01-03"
	testRecipeID         = "11111111-1111-1111-1111-111111111111"
	testMealPlanRecipeID = "22222222-2222-2222-2222-222222222222"
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

func TestRunTokenListJSON(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer pat_abc" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer pat_abc")
		}
		resp := []client.Token{
			{
				ID:        "token-1",
				Name:      "cli",
				CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runTokenList(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var got []client.Token
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("token count = %d, want 1", len(got))
	}
}

func TestRunTokenListPreflight(t *testing.T) {
	t.Parallel()

	healthCalled := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		healthCalled = true
		resp := client.HealthResponse{OK: true}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	mux.HandleFunc("/api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		resp := []client.Token{}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:       bytes.NewBufferString(""),
		stdout:      &bytes.Buffer{},
		stderr:      &bytes.Buffer{},
		store:       store,
		checkHealth: true,
	}

	exitCode := app.runTokenList(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
	if !healthCalled {
		t.Fatalf("expected health preflight to run")
	}
}

func TestRunTokenCreateWarnsOnNoExpiry(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			ID        string    `json:"id"`
			Name      string    `json:"name"`
			Token     string    `json:"token"`
			CreatedAt time.Time `json:"created_at"`
		}{
			ID:        "token-1",
			Name:      "cli",
			Token:     "pat_xyz",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: stderr,
		store:  store,
	}

	exitCode := app.runTokenCreate([]string{"--name", "cli"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("warning: token will not expire")) {
		t.Fatalf("expected warning in stderr")
	}
}

func TestRunTokenRevokeRequiresYes(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg:    config.Config{},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runTokenRevoke([]string{"token-1"})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
}

func TestRunTokenRevokeSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tokens/token-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runTokenRevoke([]string{"--yes", "token-1"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunTagListJSON(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tags", func(w http.ResponseWriter, r *http.Request) {
		resp := []client.Tag{
			{
				ID:        "tag-1",
				Name:      "Soup",
				CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runTagList(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var got []client.Tag
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("tag count = %d, want 1", len(got))
	}
}

func TestRunTagCreate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tags", func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			ID        string    `json:"id"`
			Name      string    `json:"name"`
			CreatedAt time.Time `json:"created_at"`
		}{
			ID:        "tag-1",
			Name:      "Soup",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runTagCreate([]string{"--name", "Soup"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunTagUpdate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tags/tag-1", func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			ID        string    `json:"id"`
			Name      string    `json:"name"`
			CreatedAt time.Time `json:"created_at"`
		}{
			ID:        "tag-1",
			Name:      "Dinner",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runTagUpdate([]string{"tag-1", "--name", "Dinner"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunTagDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg:    config.Config{},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runTagDelete([]string{"tag-1"})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
}

func TestRunTagDeleteSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tags/tag-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runTagDelete([]string{"--yes", "tag-1"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunBookListJSON(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipe-books", func(w http.ResponseWriter, r *http.Request) {
		resp := []client.RecipeBook{
			{
				ID:        "book-1",
				Name:      "Main",
				CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runBookList(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var got []client.RecipeBook
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("book count = %d, want 1", len(got))
	}
}

func TestRunBookCreate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipe-books", func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			ID        string    `json:"id"`
			Name      string    `json:"name"`
			CreatedAt time.Time `json:"created_at"`
		}{
			ID:        "book-1",
			Name:      "Main",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runBookCreate([]string{"--name", "Main"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunBookUpdate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipe-books/book-1", func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			ID        string    `json:"id"`
			Name      string    `json:"name"`
			CreatedAt time.Time `json:"created_at"`
		}{
			ID:        "book-1",
			Name:      "Primary",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runBookUpdate([]string{"book-1", "--name", "Primary"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunBookDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg:    config.Config{},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runBookDelete([]string{"book-1"})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
}

func TestRunBookDeleteSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipe-books/book-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runBookDelete([]string{"--yes", "book-1"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunUserListJSON(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		resp := []client.User{
			{
				ID:        "user-1",
				Username:  "sam",
				IsActive:  true,
				CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runUserList(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var got []client.User
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("user count = %d, want 1", len(got))
	}
}

func TestRunUserCreate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			ID        string    `json:"id"`
			Username  string    `json:"username"`
			IsActive  bool      `json:"is_active"`
			CreatedAt time.Time `json:"created_at"`
		}{
			ID:        "user-1",
			Username:  "sam",
			IsActive:  true,
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

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

	exitCode := app.runUserCreate([]string{"--username", "sam", "--password-stdin"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunUserDeactivateRequiresYes(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg:    config.Config{},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runUserDeactivate([]string{"user-1"})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
}

func TestRunUserDeactivateSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/users/user-1/deactivate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runUserDeactivate([]string{"--yes", "user-1"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeListJSON(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		resp := client.RecipeListResponse{
			Items: []client.RecipeListItem{
				{
					ID:               testRecipeID,
					Title:            "Soup",
					Servings:         2,
					PrepTimeMinutes:  5,
					TotalTimeMinutes: 20,
					Tags:             []client.RecipeTag{},
					UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeList(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var got client.RecipeListResponse
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items length = %d, want 1", len(got.Items))
	}
}

func TestWriteTableRecipeListNextCursor(t *testing.T) {
	t.Parallel()

	nextCursor := "cursor-1"
	resp := client.RecipeListResponse{
		Items: []client.RecipeListItem{
			{
				ID:               testRecipeID,
				Title:            "Soup",
				Servings:         2,
				PrepTimeMinutes:  5,
				TotalTimeMinutes: 20,
				Tags:             []client.RecipeTag{},
				UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		NextCursor: &nextCursor,
	}

	stdout := &bytes.Buffer{}
	exitCode := writeOutput(stdout, config.OutputTable, resp)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
	if !strings.Contains(stdout.String(), "next_cursor=cursor-1") {
		t.Fatalf("expected next_cursor output, got %q", stdout.String())
	}
}

func TestHandleAPIErrorJSON(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdout: stdout,
		stderr: stderr,
	}

	err := &client.APIError{
		StatusCode: http.StatusForbidden,
		Problem: client.Problem{
			Code:    "forbidden",
			Message: "csrf missing",
			Details: []client.FieldError{
				{Field: "csrf", Message: "required"},
			},
		},
	}

	exitCode := app.handleAPIError(err)
	if exitCode != exitForbidden {
		t.Fatalf("exit code = %d, want %d", exitCode, exitForbidden)
	}

	var got apiErrorOutput
	if decodeErr := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); decodeErr != nil {
		t.Fatalf("decode output: %v", decodeErr)
	}
	if got.Error.Status != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", got.Error.Status, http.StatusForbidden)
	}
	if got.Error.Code != "forbidden" {
		t.Fatalf("code = %q, want %q", got.Error.Code, "forbidden")
	}
	if got.Error.Message != "csrf missing" {
		t.Fatalf("message = %q, want %q", got.Error.Message, "csrf missing")
	}
	if len(got.Error.Details) != 1 || got.Error.Details[0].Field != "csrf" {
		t.Fatalf("details = %#v, want csrf field", got.Error.Details)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestHandleAPIErrorTable(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdout: stdout,
		stderr: stderr,
	}

	err := &client.APIError{
		StatusCode: http.StatusConflict,
		Problem: client.Problem{
			Code:    "conflict",
			Message: "duplicate",
		},
	}

	exitCode := app.handleAPIError(err)
	if exitCode != exitConflict {
		t.Fatalf("exit code = %d, want %d", exitCode, exitConflict)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "conflict") {
		t.Fatalf("expected stderr to include conflict, got %q", stderr.String())
	}
}

func TestRunRecipeGet(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes/"+testRecipeID, func(w http.ResponseWriter, r *http.Request) {
		resp := client.RecipeDetail{
			ID:               testRecipeID,
			Title:            "Soup",
			Servings:         2,
			PrepTimeMinutes:  5,
			TotalTimeMinutes: 20,
			Tags:             []client.RecipeTag{},
			Ingredients:      []client.RecipeIngredient{},
			Steps:            []client.RecipeStep{},
			CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:        "user-1",
			UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedBy:        "user-1",
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeGet([]string{testRecipeID})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeCreate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := client.RecipeListResponse{Items: []client.RecipeListItem{}}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		case http.MethodPost:
			resp := client.RecipeDetail{
				ID:               testRecipeID,
				Title:            "Soup",
				Servings:         2,
				PrepTimeMinutes:  5,
				TotalTimeMinutes: 20,
				Tags:             []client.RecipeTag{},
				Ingredients:      []client.RecipeIngredient{},
				Steps:            []client.RecipeStep{},
				CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				CreatedBy:        "user-1",
				UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedBy:        "user-1",
			}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	recipeJSON := []byte(`{"title":"Soup","servings":2,"prep_time_minutes":5,"total_time_minutes":20,"recipe_book_id":null,"tag_ids":[],"ingredients":[],"steps":[]}`)
	filePath := filepath.Join(t.TempDir(), "recipe.json")
	if err := os.WriteFile(filePath, recipeJSON, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeCreate([]string{"--file", filePath})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeCreateStdin(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := client.RecipeListResponse{Items: []client.RecipeListItem{}}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		case http.MethodPost:
			resp := client.RecipeDetail{
				ID:               testRecipeID,
				Title:            "Soup",
				Servings:         2,
				PrepTimeMinutes:  5,
				TotalTimeMinutes: 20,
				Tags:             []client.RecipeTag{},
				Ingredients:      []client.RecipeIngredient{},
				Steps:            []client.RecipeStep{},
				CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				CreatedBy:        "user-1",
				UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedBy:        "user-1",
			}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	recipeJSON := `{"title":"Soup","servings":2,"prep_time_minutes":5,"total_time_minutes":20,"recipe_book_id":null,"tag_ids":[],"ingredients":[],"steps":[]}`

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(recipeJSON),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeCreate([]string{"--stdin"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeCreateRejectsDuplicateTitle(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := client.RecipeListResponse{
				Items: []client.RecipeListItem{
					{
						ID:               testRecipeID,
						Title:            "Soup",
						Servings:         2,
						PrepTimeMinutes:  5,
						TotalTimeMinutes: 20,
						Tags:             []client.RecipeTag{},
						UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		case http.MethodPost:
			t.Fatalf("unexpected create call")
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	recipeJSON := []byte(`{"title":"Soup","servings":2,"prep_time_minutes":5,"total_time_minutes":20,"recipe_book_id":null,"tag_ids":[],"ingredients":[],"steps":[{"step_number":1,"instruction":"Boil"}]}`)
	filePath := filepath.Join(t.TempDir(), "recipe.json")
	if err := os.WriteFile(filePath, recipeJSON, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeCreate([]string{"--file", filePath})
	if exitCode != exitConflict {
		t.Fatalf("exit code = %d, want %d", exitCode, exitConflict)
	}
}

func TestRunRecipeUpdate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes/"+testRecipeID, func(w http.ResponseWriter, r *http.Request) {
		resp := client.RecipeDetail{
			ID:               testRecipeID,
			Title:            "Soup",
			Servings:         2,
			PrepTimeMinutes:  5,
			TotalTimeMinutes: 20,
			Tags:             []client.RecipeTag{},
			Ingredients:      []client.RecipeIngredient{},
			Steps:            []client.RecipeStep{},
			CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:        "user-1",
			UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedBy:        "user-1",
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	recipeJSON := []byte(`{"title":"Soup","servings":2,"prep_time_minutes":5,"total_time_minutes":20,"recipe_book_id":null,"tag_ids":[],"ingredients":[],"steps":[]}`)
	filePath := filepath.Join(t.TempDir(), "recipe.json")
	if err := os.WriteFile(filePath, recipeJSON, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeUpdate([]string{testRecipeID, "--file", filePath})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeUpdateStdin(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes/"+testRecipeID, func(w http.ResponseWriter, r *http.Request) {
		resp := client.RecipeDetail{
			ID:               testRecipeID,
			Title:            "Soup",
			Servings:         2,
			PrepTimeMinutes:  5,
			TotalTimeMinutes: 20,
			Tags:             []client.RecipeTag{},
			Ingredients:      []client.RecipeIngredient{},
			Steps:            []client.RecipeStep{},
			CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:        "user-1",
			UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedBy:        "user-1",
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	recipeJSON := `{"title":"Soup","servings":2,"prep_time_minutes":5,"total_time_minutes":20,"recipe_book_id":null,"tag_ids":[],"ingredients":[],"steps":[]}`

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(recipeJSON),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeUpdate([]string{testRecipeID, "--stdin"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeGetByTitle(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		resp := client.RecipeListResponse{
			Items: []client.RecipeListItem{
				{
					ID:               testRecipeID,
					Title:            "Soup",
					Servings:         2,
					PrepTimeMinutes:  5,
					TotalTimeMinutes: 20,
					Tags:             []client.RecipeTag{},
					UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	mux.HandleFunc("/api/v1/recipes/"+testRecipeID, func(w http.ResponseWriter, r *http.Request) {
		resp := client.RecipeDetail{
			ID:               testRecipeID,
			Title:            "Soup",
			Servings:         2,
			PrepTimeMinutes:  5,
			TotalTimeMinutes: 20,
			Tags:             []client.RecipeTag{},
			Ingredients:      []client.RecipeIngredient{},
			Steps:            []client.RecipeStep{{StepNumber: 1, Instruction: "Boil"}},
			CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:        "user-1",
			UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedBy:        "user-1",
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeGet([]string{"Soup"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeListFiltersByTagName(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tags", func(w http.ResponseWriter, r *http.Request) {
		resp := []client.Tag{{ID: "tag-1", Name: "Dinner", CreatedAt: time.Now()}}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	mux.HandleFunc("/api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("tag_id"); got != "tag-1" {
			t.Fatalf("tag_id = %q, want tag-1", got)
		}
		resp := client.RecipeListResponse{Items: []client.RecipeListItem{}}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeList([]string{"--tag", "Dinner"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeListWithCounts(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		resp := client.RecipeListResponse{
			Items: []client.RecipeListItem{
				{
					ID:               testRecipeID,
					Title:            "Soup",
					Servings:         2,
					PrepTimeMinutes:  5,
					TotalTimeMinutes: 20,
					Tags:             []client.RecipeTag{},
					UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	mux.HandleFunc("/api/v1/recipes/"+testRecipeID, func(w http.ResponseWriter, r *http.Request) {
		resp := client.RecipeDetail{
			ID:               testRecipeID,
			Title:            "Soup",
			Servings:         2,
			PrepTimeMinutes:  5,
			TotalTimeMinutes: 20,
			Tags:             []client.RecipeTag{},
			Ingredients:      []client.RecipeIngredient{{ID: "ing-1"}, {ID: "ing-2"}},
			Steps:            []client.RecipeStep{{StepNumber: 1, Instruction: "Boil"}},
			CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:        "user-1",
			UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedBy:        "user-1",
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeList([]string{"--with-counts"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var got recipeListWithCountsResponse
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items length = %d, want 1", len(got.Items))
	}
	if got.Items[0].IngredientCount != 2 || got.Items[0].StepCount != 1 {
		t.Fatalf("counts = %d/%d, want 2/1", got.Items[0].IngredientCount, got.Items[0].StepCount)
	}
}

func TestRunRecipeCreateInteractive(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := client.RecipeListResponse{Items: []client.RecipeListItem{}}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		case http.MethodPost:
			var payload struct {
				Title       string `json:"title"`
				Servings    int    `json:"servings"`
				Ingredients []struct {
					Item string `json:"item"`
				} `json:"ingredients"`
				Steps []struct {
					Instruction string `json:"instruction"`
				} `json:"steps"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.Title != "Soup" || payload.Servings != 2 {
				t.Fatalf("payload title/servings = %s/%d", payload.Title, payload.Servings)
			}
			if len(payload.Ingredients) != 1 || payload.Ingredients[0].Item != "Water" {
				t.Fatalf("unexpected ingredients: %#v", payload.Ingredients)
			}
			if len(payload.Steps) != 1 || payload.Steps[0].Instruction != "Boil" {
				t.Fatalf("unexpected steps: %#v", payload.Steps)
			}
			resp := client.RecipeDetail{
				ID:               testRecipeID,
				Title:            "Soup",
				Servings:         2,
				PrepTimeMinutes:  5,
				TotalTimeMinutes: 20,
				Tags:             []client.RecipeTag{},
				Ingredients:      []client.RecipeIngredient{},
				Steps:            []client.RecipeStep{},
				CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				CreatedBy:        "user-1",
				UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedBy:        "user-1",
			}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	input := strings.Join([]string{
		"Soup",
		"2",
		"5",
		"10",
		"",
		"",
		"",
		"",
		"Water",
		"",
		"Boil",
		"",
	}, "\n")

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(input),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeCreate([]string{"--interactive"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeImportArray(t *testing.T) {
	t.Parallel()

	createdIDs := []string{testRecipeID, testMealPlanRecipeID}
	callIndex := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := client.RecipeListResponse{Items: []client.RecipeListItem{}}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		case http.MethodPost:
			if callIndex >= len(createdIDs) {
				t.Fatalf("unexpected extra create call")
			}
			resp := client.RecipeDetail{
				ID:               createdIDs[callIndex],
				Title:            "Soup",
				Servings:         2,
				PrepTimeMinutes:  5,
				TotalTimeMinutes: 20,
				Tags:             []client.RecipeTag{},
				Ingredients:      []client.RecipeIngredient{},
				Steps:            []client.RecipeStep{},
				CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				CreatedBy:        "user-1",
				UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedBy:        "user-1",
			}
			callIndex++
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	payload := `[{"title":"Soup","servings":2,"prep_time_minutes":5,"total_time_minutes":20,"tag_ids":[],"ingredients":[],"steps":[{"step_number":1,"instruction":"Boil"}]},{"title":"Stew","servings":4,"prep_time_minutes":10,"total_time_minutes":30,"tag_ids":[],"ingredients":[],"steps":[{"step_number":1,"instruction":"Simmer"}]}]`

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(payload),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeImport([]string{"--stdin"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var got recipeImportResult
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items length = %d, want 2", len(got.Items))
	}
}

func TestRunRecipeTagCreatesMissing(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tags", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, []client.Tag{})
		case http.MethodPost:
			var payload struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			resp := client.Tag{
				ID:        fmt.Sprintf("tag-%s", strings.ToLower(payload.Name)),
				Name:      payload.Name,
				CreatedAt: time.Now(),
			}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})
	mux.HandleFunc("/api/v1/recipes/"+testRecipeID, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := client.RecipeDetail{
				ID:               testRecipeID,
				Title:            "Soup",
				Servings:         2,
				PrepTimeMinutes:  5,
				TotalTimeMinutes: 20,
				Tags:             []client.RecipeTag{},
				Ingredients:      []client.RecipeIngredient{},
				Steps:            []client.RecipeStep{{StepNumber: 1, Instruction: "Boil"}},
				CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				CreatedBy:        "user-1",
				UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedBy:        "user-1",
			}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		case http.MethodPut:
			var payload recipeUpsertPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if len(payload.TagIDs) != 2 {
				t.Fatalf("tag ids = %v, want 2", payload.TagIDs)
			}
			resp := client.RecipeDetail{
				ID:               testRecipeID,
				Title:            "Soup",
				Servings:         2,
				PrepTimeMinutes:  5,
				TotalTimeMinutes: 20,
				Tags: []client.RecipeTag{
					{ID: payload.TagIDs[0], Name: "Dinner"},
					{ID: payload.TagIDs[1], Name: "Quick"},
				},
				Ingredients: []client.RecipeIngredient{},
				Steps:       []client.RecipeStep{{StepNumber: 1, Instruction: "Boil"}},
				CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				CreatedBy:   "user-1",
				UpdatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedBy:   "user-1",
			}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeTag([]string{testRecipeID, "Dinner", "Quick"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeClone(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := client.RecipeListResponse{Items: []client.RecipeListItem{}}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		case http.MethodPost:
			var payload struct {
				Title string `json:"title"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.Title != "Soup (copy)" {
				t.Fatalf("title = %q, want Soup (copy)", payload.Title)
			}
			resp := client.RecipeDetail{
				ID:               testMealPlanRecipeID,
				Title:            payload.Title,
				Servings:         2,
				PrepTimeMinutes:  5,
				TotalTimeMinutes: 20,
				Tags:             []client.RecipeTag{},
				Ingredients:      []client.RecipeIngredient{},
				Steps:            []client.RecipeStep{{StepNumber: 1, Instruction: "Boil"}},
				CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				CreatedBy:        "user-1",
				UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedBy:        "user-1",
			}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})
	mux.HandleFunc("/api/v1/recipes/"+testRecipeID, func(w http.ResponseWriter, r *http.Request) {
		resp := client.RecipeDetail{
			ID:               testRecipeID,
			Title:            "Soup",
			Servings:         2,
			PrepTimeMinutes:  5,
			TotalTimeMinutes: 20,
			Tags:             []client.RecipeTag{},
			Ingredients:      []client.RecipeIngredient{},
			Steps:            []client.RecipeStep{{StepNumber: 1, Instruction: "Boil"}},
			CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:        "user-1",
			UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedBy:        "user-1",
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeClone([]string{testRecipeID})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg:    config.Config{},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runRecipeDelete([]string{testRecipeID})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
}

func TestRunRecipeDeleteSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes/"+testRecipeID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeDelete([]string{"--yes", testRecipeID})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeRestoreRequiresYes(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg:    config.Config{},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runRecipeRestore([]string{testRecipeID})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
}

func TestRunRecipeRestoreSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes/"+testRecipeID+"/restore", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeRestore([]string{"--yes", testRecipeID})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunMealPlanList(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/meal-plans", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("start") != "2025-01-01" {
			t.Fatalf("start = %q, want 2025-01-01", query.Get("start"))
		}
		if query.Get("end") != "2025-01-31" {
			t.Fatalf("end = %q, want 2025-01-31", query.Get("end"))
		}
		resp := client.MealPlanListResponse{
			Items: []client.MealPlanEntry{
				{
					Date: testMealPlanDate,
					Recipe: client.MealPlanRecipe{
						ID:    testMealPlanRecipeID,
						Title: "Pasta",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runMealPlanList([]string{"--start", "2025-01-01", "--end", "2025-01-31"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var got client.MealPlanListResponse
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(got.Items))
	}
}

func TestRunMealPlanCreate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/meal-plans", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Date     string `json:"date"`
			RecipeID string `json:"recipe_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Date != testMealPlanDate {
			t.Fatalf("date = %q, want %s", payload.Date, testMealPlanDate)
		}
		if payload.RecipeID != testMealPlanRecipeID {
			t.Fatalf("recipe_id = %q, want %s", payload.RecipeID, testMealPlanRecipeID)
		}
		resp := client.MealPlanEntry{
			Date: testMealPlanDate,
			Recipe: client.MealPlanRecipe{
				ID:    testMealPlanRecipeID,
				Title: "Pasta",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runMealPlanCreate([]string{"--date", testMealPlanDate, "--recipe-id", testMealPlanRecipeID})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunMealPlanDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg:    config.Config{},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runMealPlanDelete([]string{"--date", testMealPlanDate, "--recipe-id", testMealPlanRecipeID})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
}

func TestRunMealPlanDeleteSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/meal-plans/2025-01-03/"+testMealPlanRecipeID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runMealPlanDelete([]string{"--date", testMealPlanDate, "--recipe-id", testMealPlanRecipeID, "--yes"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeInitTemplate(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runRecipeInit(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var payload recipeUpsertPayload
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.Title == "" {
		t.Fatalf("expected title to be set")
	}
	if len(payload.Ingredients) == 0 {
		t.Fatalf("expected ingredients placeholder")
	}
	if len(payload.Steps) == 0 {
		t.Fatalf("expected steps placeholder")
	}
}

func TestRunRecipeTemplateCommand(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runRecipeTemplate(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunRecipeInitFromRecipe(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes/"+testRecipeID, func(w http.ResponseWriter, r *http.Request) {
		resp := client.RecipeDetail{
			ID:               testRecipeID,
			Title:            "Soup",
			Servings:         2,
			PrepTimeMinutes:  5,
			TotalTimeMinutes: 20,
			Tags:             []client.RecipeTag{{ID: "tag-1", Name: "Dinner"}},
			Ingredients:      []client.RecipeIngredient{},
			Steps:            []client.RecipeStep{},
			CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:        "user-1",
			UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedBy:        "user-1",
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeInit([]string{testRecipeID})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var payload recipeUpsertPayload
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(payload.TagIDs) != 1 || payload.TagIDs[0] != "tag-1" {
		t.Fatalf("unexpected tag ids: %v", payload.TagIDs)
	}
}

func TestRunRecipeExport(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes/"+testRecipeID, func(w http.ResponseWriter, r *http.Request) {
		resp := client.RecipeDetail{
			ID:               testRecipeID,
			Title:            "Soup",
			Servings:         2,
			PrepTimeMinutes:  5,
			TotalTimeMinutes: 20,
			Tags:             []client.RecipeTag{{ID: "tag-1", Name: "Dinner"}},
			Ingredients:      []client.RecipeIngredient{},
			Steps:            []client.RecipeStep{{StepNumber: 1, Instruction: "Boil"}},
			CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:        "user-1",
			UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedBy:        "user-1",
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeExport([]string{testRecipeID})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var payload recipeUpsertPayload
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&payload); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if payload.Title != "Soup" {
		t.Fatalf("title = %q, want Soup", payload.Title)
	}
}

func TestRunRecipeEditUsesEditor(t *testing.T) {
	dir := t.TempDir()
	editorPath := filepath.Join(dir, "editor.sh")
	script := `#!/bin/sh
printf '%s' '{"title":"Soup","servings":2,"prep_time_minutes":5,"total_time_minutes":20,"recipe_book_id":null,"tag_ids":[],"ingredients":[],"steps":[]}' > "$1"
`
	//nolint:gosec // Test editor script needs execute permissions.
	if err := os.WriteFile(editorPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write editor: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/recipes/"+testRecipeID, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			resp := client.RecipeDetail{
				ID:               testRecipeID,
				Title:            "Soup",
				Servings:         2,
				PrepTimeMinutes:  5,
				TotalTimeMinutes: 20,
				Tags:             []client.RecipeTag{},
				Ingredients:      []client.RecipeIngredient{},
				Steps:            []client.RecipeStep{},
				CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				CreatedBy:        "user-1",
				UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedBy:        "user-1",
			}
			w.Header().Set("Content-Type", "application/json")
			writeTestJSON(t, w, resp)
		case http.MethodPut:
			w.Header().Set("Content-Type", "application/json")
			resp := client.RecipeDetail{
				ID:               testRecipeID,
				Title:            "Soup",
				Servings:         2,
				PrepTimeMinutes:  5,
				TotalTimeMinutes: 20,
				Tags:             []client.RecipeTag{},
				Ingredients:      []client.RecipeIngredient{},
				Steps:            []client.RecipeStep{},
				CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				CreatedBy:        "user-1",
				UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				UpdatedBy:        "user-1",
			}
			writeTestJSON(t, w, resp)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	credsPath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(credsPath)
	if err := store.Save(credentials.Credentials{Token: "pat_abc"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	t.Setenv("EDITOR", editorPath)

	app := &App{
		cfg: config.Config{
			APIURL:  server.URL,
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  store,
	}

	exitCode := app.runRecipeEdit([]string{testRecipeID})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunConfigSetAndView(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: stderr,
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runConfigSet([]string{
		"--config", configPath,
		"--api-url", "http://example.test",
		"--output", "json",
		"--timeout", "45s",
		"--debug",
	})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	stdout.Reset()
	exitCode = app.runConfigView([]string{"--config", configPath})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var view configView
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&view); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if view.APIURL != "http://example.test" {
		t.Fatalf("api url = %q, want %q", view.APIURL, "http://example.test")
	}
	if view.Output != "json" {
		t.Fatalf("output = %q, want json", view.Output)
	}
	if view.Timeout != "45s" {
		t.Fatalf("timeout = %q, want 45s", view.Timeout)
	}
	if view.Debug != true {
		t.Fatalf("debug = %t, want true", view.Debug)
	}
}

func TestRunConfigUnset(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := config.Save(configPath, config.Config{
		APIURL:  "http://example.test",
		Output:  config.OutputJSON,
		Timeout: 45 * time.Second,
		Debug:   true,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runConfigUnset([]string{"--config", configPath, "--api-url", "--debug"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var view configView
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&view); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if view.APIURL != "http://localhost:8080" {
		t.Fatalf("api url = %q, want default", view.APIURL)
	}
	if view.Debug != false {
		t.Fatalf("debug = %t, want false", view.Debug)
	}
}

func TestRunConfigPath(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runConfigPath(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var view configView
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&view); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if view.ConfigPath == "" {
		t.Fatalf("expected config path")
	}
}

func TestRunVersionJSON(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalBuiltAt := BuiltAt
	Version = testVersion
	Commit = testCommit
	BuiltAt = testBuiltAt
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		BuiltAt = originalBuiltAt
	})

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runVersion(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var info versionInfo
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&info); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if info.Version != testVersion {
		t.Fatalf("version = %q, want %q", info.Version, testVersion)
	}
	if info.Commit != testCommit {
		t.Fatalf("commit = %q, want %q", info.Commit, testCommit)
	}
	if info.BuiltAt != testBuiltAt {
		t.Fatalf("built_at = %q, want %q", info.BuiltAt, testBuiltAt)
	}
}

func TestRunVersionTable(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalBuiltAt := BuiltAt
	Version = testVersion
	Commit = testCommit
	BuiltAt = testBuiltAt
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		BuiltAt = originalBuiltAt
	})

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runVersion(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	output := stdout.String()
	if !strings.Contains(output, "version") || !strings.Contains(output, testVersion) {
		t.Fatalf("expected version output, got %q", output)
	}
	if !strings.Contains(output, "commit") || !strings.Contains(output, testCommit) {
		t.Fatalf("expected commit output, got %q", output)
	}
	if !strings.Contains(output, "built_at") || !strings.Contains(output, testBuiltAt) {
		t.Fatalf("expected built_at output, got %q", output)
	}
}

func TestRunVersionFlag(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalBuiltAt := BuiltAt
	Version = testVersion
	Commit = testCommit
	BuiltAt = testBuiltAt
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		BuiltAt = originalBuiltAt
	})

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("COOKING_OUTPUT", "json")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := Run([]string{"cookctl", "--version"}, bytes.NewBufferString(""), stdout, stderr)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var info versionInfo
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&info); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if info.Version != testVersion {
		t.Fatalf("version = %q, want %q", info.Version, testVersion)
	}
	if info.Commit != testCommit {
		t.Fatalf("commit = %q, want %q", info.Commit, testCommit)
	}
	if info.BuiltAt != testBuiltAt {
		t.Fatalf("built_at = %q, want %q", info.BuiltAt, testBuiltAt)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunGlobalFlagsAfterCommand(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalBuiltAt := BuiltAt
	Version = testVersion
	Commit = testCommit
	BuiltAt = testBuiltAt
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		BuiltAt = originalBuiltAt
	})

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := Run([]string{"cookctl", "version", "--output", "json"}, bytes.NewBufferString(""), stdout, stderr)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var info versionInfo
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&info); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if info.Version != testVersion {
		t.Fatalf("version = %q, want %q", info.Version, testVersion)
	}
	if info.Commit != testCommit {
		t.Fatalf("commit = %q, want %q", info.Commit, testCommit)
	}
	if info.BuiltAt != testBuiltAt {
		t.Fatalf("built_at = %q, want %q", info.BuiltAt, testBuiltAt)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunCompletionBash(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runCompletion([]string{"bash"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	output := stdout.String()
	if !strings.Contains(output, "complete -F _cookctl cookctl") {
		t.Fatalf("expected bash completion output, got %q", output)
	}
	if !strings.Contains(output, "auth") {
		t.Fatalf("expected auth completion entry, got %q", output)
	}
}

func TestSplitGlobalArgsInterspersed(t *testing.T) {
	t.Parallel()

	globalArgs, commandArgs, err := splitGlobalArgs([]string{
		"recipe",
		"list",
		"--output",
		"json",
		"--timeout=15s",
		"--debug",
	})
	if err != nil {
		t.Fatalf("split global args: %v", err)
	}

	expectedGlobals := []string{"--output", "json", "--timeout=15s", "--debug"}
	if !reflect.DeepEqual(globalArgs, expectedGlobals) {
		t.Fatalf("global args = %v, want %v", globalArgs, expectedGlobals)
	}

	expectedCommands := []string{"recipe", "list"}
	if !reflect.DeepEqual(commandArgs, expectedCommands) {
		t.Fatalf("command args = %v, want %v", commandArgs, expectedCommands)
	}
}

func TestSplitGlobalArgsHelpAfterCommand(t *testing.T) {
	t.Parallel()

	globalArgs, commandArgs, err := splitGlobalArgs([]string{
		"recipe",
		"list",
		"--help",
		"--output",
		"json",
	})
	if err != nil {
		t.Fatalf("split global args: %v", err)
	}

	expectedGlobals := []string{"--output", "json"}
	if !reflect.DeepEqual(globalArgs, expectedGlobals) {
		t.Fatalf("global args = %v, want %v", globalArgs, expectedGlobals)
	}

	expectedCommands := []string{"recipe", "list", "--help"}
	if !reflect.DeepEqual(commandArgs, expectedCommands) {
		t.Fatalf("command args = %v, want %v", commandArgs, expectedCommands)
	}
}

func TestSplitGlobalArgsMissingValue(t *testing.T) {
	t.Parallel()

	_, _, err := splitGlobalArgs([]string{"recipe", "list", "--output"})
	if err == nil {
		t.Fatalf("expected error for missing flag value")
	}
}

func TestRunCompletionZsh(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runCompletion([]string{"zsh"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	output := stdout.String()
	if !strings.Contains(output, "#compdef cookctl") {
		t.Fatalf("expected zsh completion output, got %q", output)
	}
	if !strings.Contains(output, "completion") {
		t.Fatalf("expected completion entry, got %q", output)
	}
}

func TestRunCompletionFish(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runCompletion([]string{"fish"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	output := stdout.String()
	if !strings.Contains(output, "complete -c cookctl") {
		t.Fatalf("expected fish completion output, got %q", output)
	}
	if !strings.Contains(output, "recipe") {
		t.Fatalf("expected recipe completion entry, got %q", output)
	}
}

func TestRunCompletionInvalidShell(t *testing.T) {
	t.Parallel()

	stderr := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: stderr,
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runCompletion([]string{"powershell"})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
	if !strings.Contains(stderr.String(), "unsupported shell") {
		t.Fatalf("expected unsupported shell error, got %q", stderr.String())
	}
}

func TestRunHelpGeneral(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runHelp(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
	if !strings.Contains(stdout.String(), "usage: cookctl") {
		t.Fatalf("expected usage output, got %q", stdout.String())
	}
}

func TestRunHelpAuth(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runHelp([]string{"auth"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
	if !strings.Contains(stdout.String(), "cookctl auth") {
		t.Fatalf("expected auth usage output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "--token-stdin") {
		t.Fatalf("expected auth usage to include token-stdin, got %q", stdout.String())
	}
}

func TestRunHelpUnknownTopic(t *testing.T) {
	t.Parallel()

	stderr := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: stderr,
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runHelp([]string{"nope"})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
	if !strings.Contains(stderr.String(), "unknown help topic") {
		t.Fatalf("expected help topic error, got %q", stderr.String())
	}
}

func TestRunHelpFlagGeneral(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := Run([]string{"cookctl", "--help"}, bytes.NewBufferString(""), stdout, stderr)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
	if !strings.Contains(stdout.String(), "usage: cookctl") {
		t.Fatalf("expected usage output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunHelpFlagTopic(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := Run([]string{"cookctl", "--help", "auth"}, bytes.NewBufferString(""), stdout, stderr)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
	if !strings.Contains(stdout.String(), "cookctl auth") {
		t.Fatalf("expected auth usage output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunHelpFlagSubcommand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := Run([]string{"cookctl", "recipe", "list", "--help"}, bytes.NewBufferString(""), stdout, stderr)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
	if !strings.Contains(stdout.String(), "usage: cookctl recipe list") {
		t.Fatalf("expected recipe list usage output, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
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

func TestRunTokenListUsesStoredAPIURL(t *testing.T) {
	t.Parallel()

	defaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to default server: %s", r.URL.Path)
	}))
	t.Cleanup(defaultServer.Close)

	customServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tokens" {
			t.Fatalf("path = %s, want /api/v1/tokens", r.URL.Path)
		}
		resp := []client.Token{
			{
				ID:        "token-1",
				Name:      "cli",
				CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	}))
	t.Cleanup(customServer.Close)

	storePath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(storePath)
	if err := store.Save(credentials.Credentials{
		Token:  "pat_123",
		APIURL: customServer.URL,
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
			APIURL:  defaultServer.URL,
		},
		stdin:          bytes.NewBufferString(""),
		stdout:         stdout,
		stderr:         &bytes.Buffer{},
		store:          store,
		apiURLOverride: false,
	}

	exitCode := app.runTokenList(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var tokens []client.Token
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&tokens); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(tokens) != 1 || tokens[0].ID != "token-1" {
		t.Fatalf("unexpected tokens: %+v", tokens)
	}
}

func TestRunTokenListUsesOverrideAPIURL(t *testing.T) {
	t.Parallel()

	storedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to stored server: %s", r.URL.Path)
	}))
	t.Cleanup(storedServer.Close)

	overrideServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tokens" {
			t.Fatalf("path = %s, want /api/v1/tokens", r.URL.Path)
		}
		resp := []client.Token{
			{
				ID:        "token-override",
				Name:      "cli",
				CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	}))
	t.Cleanup(overrideServer.Close)

	storePath := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewStore(storePath)
	if err := store.Save(credentials.Credentials{
		Token:  "pat_123",
		APIURL: storedServer.URL,
	}); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
			APIURL:  overrideServer.URL,
		},
		stdin:          bytes.NewBufferString(""),
		stdout:         stdout,
		stderr:         &bytes.Buffer{},
		store:          store,
		apiURLOverride: true,
	}

	exitCode := app.runTokenList(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var tokens []client.Token
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&tokens); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(tokens) != 1 || tokens[0].ID != "token-override" {
		t.Fatalf("unexpected tokens: %+v", tokens)
	}
}
