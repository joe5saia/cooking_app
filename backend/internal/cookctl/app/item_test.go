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

func TestRunItemListJSON(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/items", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "milk" {
			t.Fatalf("q = %q, want milk", got)
		}
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Fatalf("limit = %q, want 5", got)
		}
		resp := []client.Item{
			{ID: "item-1", Name: "milk"},
		}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	store := credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json"))
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

	exitCode := app.runItemList([]string{"--q", "milk", "--limit", "5"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var got []client.Item
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("item count = %d, want 1", len(got))
	}
}

func TestRunItemCreate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/items", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Name     string  `json:"name"`
			StoreURL *string `json:"store_url"`
			AisleID  *string `json:"aisle_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Name != "milk" {
			t.Fatalf("name = %q, want milk", payload.Name)
		}
		resp := client.Item{ID: "item-1", Name: payload.Name}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	store := credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json"))
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

	exitCode := app.runItemCreate([]string{"--name", "milk", "--store-url", "https://shop.example/milk"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunItemUpdate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/items/item-1", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Name != "skim milk" {
			t.Fatalf("name = %q, want skim milk", payload.Name)
		}
		resp := client.Item{ID: "item-1", Name: payload.Name}
		w.Header().Set("Content-Type", "application/json")
		writeTestJSON(t, w, resp)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	store := credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json"))
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

	exitCode := app.runItemUpdate([]string{"item-1", "--name", "skim milk"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunItemDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg:    config.Config{},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runItemDelete([]string{"item-1"})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
}

func TestRunItemDeleteSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/items/item-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	store := credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json"))
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

	exitCode := app.runItemDelete([]string{"--yes", "item-1"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}
