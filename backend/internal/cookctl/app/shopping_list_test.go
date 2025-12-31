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

const (
	testShoppingListID   = "list-1"
	testShoppingListDate = "2025-02-01"
	testShoppingItemID   = "list-item-1"
)

func TestRunShoppingListList(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shopping-lists", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("start") != "2025-02-01" {
			t.Fatalf("start = %q, want 2025-02-01", query.Get("start"))
		}
		if query.Get("end") != "2025-02-28" {
			t.Fatalf("end = %q, want 2025-02-28", query.Get("end"))
		}
		resp := []client.ShoppingList{
			{ID: testShoppingListID, Name: "Weekly shop", ListDate: testShoppingListDate},
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

	exitCode := app.runShoppingListList([]string{"--start", "2025-02-01", "--end", "2025-02-28"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var got []client.ShoppingList
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("shopping lists len = %d, want 1", len(got))
	}
}

func TestRunShoppingListCreate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shopping-lists", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			ListDate string  `json:"list_date"`
			Name     string  `json:"name"`
			Notes    *string `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.ListDate != testShoppingListDate {
			t.Fatalf("list_date = %q, want %s", payload.ListDate, testShoppingListDate)
		}
		if payload.Name != "Weekly shop" {
			t.Fatalf("name = %q, want Weekly shop", payload.Name)
		}
		if payload.Notes == nil || *payload.Notes != "Notes" {
			t.Fatalf("notes = %v, want Notes", payload.Notes)
		}
		resp := client.ShoppingList{ID: testShoppingListID, Name: payload.Name, ListDate: payload.ListDate}
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

	exitCode := app.runShoppingListCreate([]string{"--date", testShoppingListDate, "--name", "Weekly shop", "--notes", "Notes"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunShoppingListGet(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shopping-lists/"+testShoppingListID, func(w http.ResponseWriter, r *http.Request) {
		resp := client.ShoppingListDetail{
			ID:       testShoppingListID,
			Name:     "Weekly shop",
			ListDate: testShoppingListDate,
			Items:    []client.ShoppingListItem{},
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

	exitCode := app.runShoppingListGet([]string{testShoppingListID})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunShoppingListUpdate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shopping-lists/"+testShoppingListID, func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			ListDate string  `json:"list_date"`
			Name     string  `json:"name"`
			Notes    *string `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.ListDate != testShoppingListDate {
			t.Fatalf("list_date = %q, want %s", payload.ListDate, testShoppingListDate)
		}
		if payload.Name != "Updated list" {
			t.Fatalf("name = %q, want Updated list", payload.Name)
		}
		resp := client.ShoppingList{ID: testShoppingListID, Name: payload.Name, ListDate: payload.ListDate}
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

	exitCode := app.runShoppingListUpdate([]string{testShoppingListID, "--date", testShoppingListDate, "--name", "Updated list"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunShoppingListDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg:    config.Config{},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runShoppingListDelete([]string{testShoppingListID})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
}

func TestRunShoppingListDeleteSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shopping-lists/"+testShoppingListID, func(w http.ResponseWriter, r *http.Request) {
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

	exitCode := app.runShoppingListDelete([]string{"--yes", testShoppingListID})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunShoppingListItemsList(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shopping-lists/"+testShoppingListID+"/items", func(w http.ResponseWriter, r *http.Request) {
		resp := []client.ShoppingListItem{
			{ID: testShoppingItemID, Item: client.Item{ID: "item-1", Name: "Milk"}},
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

	exitCode := app.runShoppingListItemsList([]string{testShoppingListID})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var got []client.ShoppingListItem
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("items len = %d, want 1", len(got))
	}
}

func TestRunShoppingListItemsAdd(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shopping-lists/"+testShoppingListID+"/items", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Items []client.ShoppingListItemInput `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Items) != 1 {
			t.Fatalf("items len = %d, want 1", len(payload.Items))
		}
		if payload.Items[0].ItemID != "item-1" {
			t.Fatalf("item_id = %q, want item-1", payload.Items[0].ItemID)
		}
		if payload.Items[0].Quantity == nil || *payload.Items[0].Quantity != 0 {
			t.Fatalf("quantity = %v, want 0", payload.Items[0].Quantity)
		}
		resp := []client.ShoppingListItem{
			{ID: testShoppingItemID, Item: client.Item{ID: payload.Items[0].ItemID, Name: "Milk"}},
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

	exitCode := app.runShoppingListItemsAdd([]string{testShoppingListID, "--item-id", "item-1", "--quantity", "0"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunShoppingListItemsFromRecipes(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shopping-lists/"+testShoppingListID+"/items/from-recipes", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			RecipeIDs []string `json:"recipe_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.RecipeIDs) != 2 {
			t.Fatalf("recipe_ids len = %d, want 2", len(payload.RecipeIDs))
		}
		resp := []client.ShoppingListItem{{ID: testShoppingItemID, Item: client.Item{ID: "item-1", Name: "Milk"}}}
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

	exitCode := app.runShoppingListItemsFromRecipes([]string{testShoppingListID, "--recipe-id", "recipe-1", "--recipe-id", "recipe-2"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunShoppingListItemsFromMealPlan(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shopping-lists/"+testShoppingListID+"/items/from-meal-plan", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Date string `json:"date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Date != testShoppingListDate {
			t.Fatalf("date = %q, want %s", payload.Date, testShoppingListDate)
		}
		resp := []client.ShoppingListItem{{ID: testShoppingItemID, Item: client.Item{ID: "item-1", Name: "Milk"}}}
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

	exitCode := app.runShoppingListItemsFromMealPlan([]string{testShoppingListID, "--date", testShoppingListDate})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunShoppingListItemsPurchase(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shopping-lists/"+testShoppingListID+"/items/"+testShoppingItemID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s, want PATCH", r.Method)
		}
		var payload struct {
			IsPurchased bool `json:"is_purchased"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if !payload.IsPurchased {
			t.Fatalf("is_purchased = false, want true")
		}
		resp := client.ShoppingListItem{ID: testShoppingItemID, Item: client.Item{ID: "item-1", Name: "Milk"}, IsPurchased: true}
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

	exitCode := app.runShoppingListItemsPurchase([]string{"--list-id", testShoppingListID, "--item-id", testShoppingItemID, "--purchased"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}

func TestRunShoppingListItemsDeleteRequiresYes(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg:    config.Config{},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runShoppingListItemsDelete([]string{"--list-id", testShoppingListID, "--item-id", testShoppingItemID})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
}

func TestRunShoppingListItemsDeleteSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/shopping-lists/"+testShoppingListID+"/items/"+testShoppingItemID, func(w http.ResponseWriter, r *http.Request) {
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

	exitCode := app.runShoppingListItemsDelete([]string{"--list-id", testShoppingListID, "--item-id", testShoppingItemID, "--yes"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
}
