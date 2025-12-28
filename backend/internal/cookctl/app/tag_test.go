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
