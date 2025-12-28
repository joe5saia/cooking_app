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
