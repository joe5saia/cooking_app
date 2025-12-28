package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/config"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/credentials"
)

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
					ItemName string `json:"item_name"`
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
			if len(payload.Ingredients) != 1 || payload.Ingredients[0].ItemName != "Water" {
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
