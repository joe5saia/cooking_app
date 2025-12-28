package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/config"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/credentials"
)

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
