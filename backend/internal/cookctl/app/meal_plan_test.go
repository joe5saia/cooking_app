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
