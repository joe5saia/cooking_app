package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const (
	testMealPlanDate     = "2025-01-03"
	testMealPlanRecipeID = "recipe-1"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/healthz" {
			t.Fatalf("path = %s, want /api/v1/healthz", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, HealthResponse{OK: true})
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.Health(context.Background())
	if err != nil {
		t.Fatalf("Health returned error: %v", err)
	}
	if resp.OK != true {
		t.Fatalf("OK = %t, want true", resp.OK)
	}
}

func TestMeAuthHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer pat_456" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer pat_456")
		}
		resp := MeResponse{
			ID:       "user-1",
			Username: "sam",
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if _, err := api.Me(context.Background()); err != nil {
		t.Fatalf("Me returned error: %v", err)
	}
}

func TestAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(t, w, Problem{
			Code:    "unauthorized",
			Message: "nope",
		})
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = api.Me(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
	if apiErr.Problem.Code != "unauthorized" {
		t.Fatalf("Problem.Code = %q, want %q", apiErr.Problem.Code, "unauthorized")
	}
}

func TestTokens(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tokens" {
			t.Fatalf("path = %s, want /api/v1/tokens", r.URL.Path)
		}
		resp := []Token{
			{
				ID:        "token-1",
				Name:      "cli",
				CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.Tokens(context.Background())
	if err != nil {
		t.Fatalf("Tokens returned error: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("Tokens length = %d, want 1", len(resp))
	}
}

func TestCreateToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tokens" {
			t.Fatalf("path = %s, want /api/v1/tokens", r.URL.Path)
		}
		var payload struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Name != "cli" {
			t.Fatalf("name = %q, want cli", payload.Name)
		}
		resp := CreateTokenResponse{
			ID:        "token-1",
			Name:      "cli",
			Token:     "pat_xyz",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.CreateToken(context.Background(), "cli", nil)
	if err != nil {
		t.Fatalf("CreateToken returned error: %v", err)
	}
	if resp.Token != "pat_xyz" {
		t.Fatalf("Token = %q, want %q", resp.Token, "pat_xyz")
	}
}

func TestRevokeToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/v1/tokens/token-1" {
			t.Fatalf("path = %s, want /api/v1/tokens/token-1", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := api.RevokeToken(context.Background(), "token-1"); err != nil {
		t.Fatalf("RevokeToken returned error: %v", err)
	}
}

func TestTags(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tags" {
			t.Fatalf("path = %s, want /api/v1/tags", r.URL.Path)
		}
		resp := []Tag{
			{
				ID:        "tag-1",
				Name:      "Soup",
				CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.Tags(context.Background())
	if err != nil {
		t.Fatalf("Tags returned error: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("Tags length = %d, want 1", len(resp))
	}
}

func TestCreateTag(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tags" {
			t.Fatalf("path = %s, want /api/v1/tags", r.URL.Path)
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["name"] != "Soup" {
			t.Fatalf("name = %v, want Soup", payload["name"])
		}
		resp := Tag{
			ID:        "tag-1",
			Name:      "Soup",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.CreateTag(context.Background(), "Soup")
	if err != nil {
		t.Fatalf("CreateTag returned error: %v", err)
	}
	if resp.ID != "tag-1" {
		t.Fatalf("ID = %q, want %q", resp.ID, "tag-1")
	}
}

func TestUpdateTag(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/api/v1/tags/tag-1" {
			t.Fatalf("path = %s, want /api/v1/tags/tag-1", r.URL.Path)
		}
		resp := Tag{
			ID:        "tag-1",
			Name:      "Dinner",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.UpdateTag(context.Background(), "tag-1", "Dinner")
	if err != nil {
		t.Fatalf("UpdateTag returned error: %v", err)
	}
	if resp.Name != "Dinner" {
		t.Fatalf("Name = %q, want %q", resp.Name, "Dinner")
	}
}

func TestDeleteTag(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/v1/tags/tag-1" {
			t.Fatalf("path = %s, want /api/v1/tags/tag-1", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := api.DeleteTag(context.Background(), "tag-1"); err != nil {
		t.Fatalf("DeleteTag returned error: %v", err)
	}
}

func TestRecipeBooks(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/recipe-books" {
			t.Fatalf("path = %s, want /api/v1/recipe-books", r.URL.Path)
		}
		resp := []RecipeBook{
			{
				ID:        "book-1",
				Name:      "Main",
				CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.RecipeBooks(context.Background())
	if err != nil {
		t.Fatalf("RecipeBooks returned error: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("RecipeBooks length = %d, want 1", len(resp))
	}
}

func TestCreateRecipeBook(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/recipe-books" {
			t.Fatalf("path = %s, want /api/v1/recipe-books", r.URL.Path)
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["name"] != "Main" {
			t.Fatalf("name = %v, want Main", payload["name"])
		}
		resp := RecipeBook{
			ID:        "book-1",
			Name:      "Main",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.CreateRecipeBook(context.Background(), "Main")
	if err != nil {
		t.Fatalf("CreateRecipeBook returned error: %v", err)
	}
	if resp.ID != "book-1" {
		t.Fatalf("ID = %q, want %q", resp.ID, "book-1")
	}
}

func TestUpdateRecipeBook(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/api/v1/recipe-books/book-1" {
			t.Fatalf("path = %s, want /api/v1/recipe-books/book-1", r.URL.Path)
		}
		resp := RecipeBook{
			ID:        "book-1",
			Name:      "Primary",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.UpdateRecipeBook(context.Background(), "book-1", "Primary")
	if err != nil {
		t.Fatalf("UpdateRecipeBook returned error: %v", err)
	}
	if resp.Name != "Primary" {
		t.Fatalf("Name = %q, want %q", resp.Name, "Primary")
	}
}

func TestDeleteRecipeBook(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/v1/recipe-books/book-1" {
			t.Fatalf("path = %s, want /api/v1/recipe-books/book-1", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := api.DeleteRecipeBook(context.Background(), "book-1"); err != nil {
		t.Fatalf("DeleteRecipeBook returned error: %v", err)
	}
}

func TestUsers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/users" {
			t.Fatalf("path = %s, want /api/v1/users", r.URL.Path)
		}
		resp := []User{
			{
				ID:        "user-1",
				Username:  "sam",
				IsActive:  true,
				CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.Users(context.Background())
	if err != nil {
		t.Fatalf("Users returned error: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("Users length = %d, want 1", len(resp))
	}
}

func TestCreateUser(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/users" {
			t.Fatalf("path = %s, want /api/v1/users", r.URL.Path)
		}
		var payload struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Username != "sam" {
			t.Fatalf("username = %q, want sam", payload.Username)
		}
		if payload.Password != "pw" {
			t.Fatalf("password = %q, want pw", payload.Password)
		}
		resp := User{
			ID:        "user-1",
			Username:  "sam",
			IsActive:  true,
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.CreateUser(context.Background(), "sam", "pw", nil)
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if resp.ID != "user-1" {
		t.Fatalf("ID = %q, want %q", resp.ID, "user-1")
	}
}

func TestDeactivateUser(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/api/v1/users/user-1/deactivate" {
			t.Fatalf("path = %s, want /api/v1/users/user-1/deactivate", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := api.DeactivateUser(context.Background(), "user-1"); err != nil {
		t.Fatalf("DeactivateUser returned error: %v", err)
	}
}

func TestRecipesList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/recipes" {
			t.Fatalf("path = %s, want /api/v1/recipes", r.URL.Path)
		}
		resp := RecipeListResponse{
			Items: []RecipeListItem{
				{
					ID:               "recipe-1",
					Title:            "Soup",
					Servings:         2,
					PrepTimeMinutes:  5,
					TotalTimeMinutes: 20,
					Tags:             []RecipeTag{{ID: "tag-1", Name: "Dinner"}},
					UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.Recipes(context.Background(), RecipeListParams{Query: "Soup"})
	if err != nil {
		t.Fatalf("Recipes returned error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("Items length = %d, want 1", len(resp.Items))
	}
}

func TestMealPlansList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/meal-plans" {
			t.Fatalf("path = %s, want /api/v1/meal-plans", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("start") != "2025-01-01" {
			t.Fatalf("start = %s, want 2025-01-01", query.Get("start"))
		}
		if query.Get("end") != "2025-01-31" {
			t.Fatalf("end = %s, want 2025-01-31", query.Get("end"))
		}
		resp := MealPlanListResponse{
			Items: []MealPlanEntry{
				{
					Date: testMealPlanDate,
					Recipe: MealPlanRecipe{
						ID:    testMealPlanRecipeID,
						Title: "Soup",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.MealPlans(context.Background(), "2025-01-01", "2025-01-31")
	if err != nil {
		t.Fatalf("MealPlans returned error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("Items length = %d, want 1", len(resp.Items))
	}
}

func TestCreateMealPlan(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/meal-plans" {
			t.Fatalf("path = %s, want /api/v1/meal-plans", r.URL.Path)
		}
		var payload struct {
			Date     string `json:"date"`
			RecipeID string `json:"recipe_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Date != testMealPlanDate {
			t.Fatalf("date = %s, want %s", payload.Date, testMealPlanDate)
		}
		if payload.RecipeID != testMealPlanRecipeID {
			t.Fatalf("recipe_id = %s, want %s", payload.RecipeID, testMealPlanRecipeID)
		}
		resp := MealPlanEntry{
			Date: testMealPlanDate,
			Recipe: MealPlanRecipe{
				ID:    testMealPlanRecipeID,
				Title: "Soup",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.CreateMealPlan(context.Background(), testMealPlanDate, testMealPlanRecipeID)
	if err != nil {
		t.Fatalf("CreateMealPlan returned error: %v", err)
	}
	if resp.Recipe.ID != testMealPlanRecipeID {
		t.Fatalf("Recipe ID = %q, want %s", resp.Recipe.ID, testMealPlanRecipeID)
	}
}

func TestDeleteMealPlan(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/v1/meal-plans/2025-01-03/recipe-1" {
			t.Fatalf("path = %s, want /api/v1/meal-plans/2025-01-03/recipe-1", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := api.DeleteMealPlan(context.Background(), testMealPlanDate, testMealPlanRecipeID); err != nil {
		t.Fatalf("DeleteMealPlan returned error: %v", err)
	}
}

func TestRecipeDetail(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/recipes/recipe-1" {
			t.Fatalf("path = %s, want /api/v1/recipes/recipe-1", r.URL.Path)
		}
		resp := RecipeDetail{
			ID:               "recipe-1",
			Title:            "Soup",
			Servings:         2,
			PrepTimeMinutes:  5,
			TotalTimeMinutes: 20,
			Tags:             []RecipeTag{{ID: "tag-1", Name: "Dinner"}},
			Ingredients:      []RecipeIngredient{},
			Steps:            []RecipeStep{},
			CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:        "user-1",
			UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedBy:        "user-1",
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.Recipe(context.Background(), "recipe-1")
	if err != nil {
		t.Fatalf("Recipe returned error: %v", err)
	}
	if resp.ID != "recipe-1" {
		t.Fatalf("ID = %q, want %q", resp.ID, "recipe-1")
	}
}

func TestCreateRecipe(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		resp := RecipeDetail{
			ID:               "recipe-1",
			Title:            "Soup",
			Servings:         2,
			PrepTimeMinutes:  5,
			TotalTimeMinutes: 20,
			Tags:             []RecipeTag{},
			Ingredients:      []RecipeIngredient{},
			Steps:            []RecipeStep{},
			CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:        "user-1",
			UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedBy:        "user-1",
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	payload := json.RawMessage(`{"title":"Soup","servings":2,"prep_time_minutes":5,"total_time_minutes":20,"recipe_book_id":null,"tag_ids":[],"ingredients":[],"steps":[]}`)
	resp, err := api.CreateRecipe(context.Background(), payload)
	if err != nil {
		t.Fatalf("CreateRecipe returned error: %v", err)
	}
	if resp.Title != "Soup" {
		t.Fatalf("Title = %q, want Soup", resp.Title)
	}
}

func TestUpdateRecipe(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		resp := RecipeDetail{
			ID:               "recipe-1",
			Title:            "Soup",
			Servings:         2,
			PrepTimeMinutes:  5,
			TotalTimeMinutes: 20,
			Tags:             []RecipeTag{},
			Ingredients:      []RecipeIngredient{},
			Steps:            []RecipeStep{},
			CreatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			CreatedBy:        "user-1",
			UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedBy:        "user-1",
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	payload := json.RawMessage(`{"title":"Soup","servings":2,"prep_time_minutes":5,"total_time_minutes":20,"recipe_book_id":null,"tag_ids":[],"ingredients":[],"steps":[]}`)
	if _, err := api.UpdateRecipe(context.Background(), "recipe-1", payload); err != nil {
		t.Fatalf("UpdateRecipe returned error: %v", err)
	}
}

func TestDeleteRecipe(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := api.DeleteRecipe(context.Background(), "recipe-1"); err != nil {
		t.Fatalf("DeleteRecipe returned error: %v", err)
	}
}

func TestRestoreRecipe(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := api.RestoreRecipe(context.Background(), "recipe-1"); err != nil {
		t.Fatalf("RestoreRecipe returned error: %v", err)
	}
}

func TestItemsList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/items" {
			t.Fatalf("path = %s, want /api/v1/items", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "milk" {
			t.Fatalf("q = %q, want milk", got)
		}
		if got := r.URL.Query().Get("limit"); got != "25" {
			t.Fatalf("limit = %q, want 25", got)
		}
		resp := []Item{
			{
				ID:   "item-1",
				Name: "milk",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.Items(context.Background(), ItemListParams{Query: "milk", Limit: 25})
	if err != nil {
		t.Fatalf("Items returned error: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("Items length = %d, want 1", len(resp))
	}
}

func TestCreateItem(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/items" {
			t.Fatalf("path = %s, want /api/v1/items", r.URL.Path)
		}
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
		if payload.StoreURL == nil || *payload.StoreURL != "https://shop.example/milk" {
			t.Fatalf("store_url = %v, want https://shop.example/milk", payload.StoreURL)
		}
		if payload.AisleID == nil || *payload.AisleID != "aisle-1" {
			t.Fatalf("aisle_id = %v, want aisle-1", payload.AisleID)
		}
		resp := Item{ID: "item-1", Name: "milk"}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.CreateItem(context.Background(), "milk", stringPtr("https://shop.example/milk"), stringPtr("aisle-1"))
	if err != nil {
		t.Fatalf("CreateItem returned error: %v", err)
	}
	if resp.ID != "item-1" {
		t.Fatalf("ID = %q, want item-1", resp.ID)
	}
}

func TestUpdateItem(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/api/v1/items/item-1" {
			t.Fatalf("path = %s, want /api/v1/items/item-1", r.URL.Path)
		}
		var payload struct {
			Name     string  `json:"name"`
			StoreURL *string `json:"store_url"`
			AisleID  *string `json:"aisle_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Name != "skim milk" {
			t.Fatalf("name = %q, want skim milk", payload.Name)
		}
		resp := Item{ID: "item-1", Name: payload.Name}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.UpdateItem(context.Background(), "item-1", "skim milk", nil, nil)
	if err != nil {
		t.Fatalf("UpdateItem returned error: %v", err)
	}
	if resp.Name != "skim milk" {
		t.Fatalf("Name = %q, want skim milk", resp.Name)
	}
}

func TestDeleteItem(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/v1/items/item-1" {
			t.Fatalf("path = %s, want /api/v1/items/item-1", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := api.DeleteItem(context.Background(), "item-1"); err != nil {
		t.Fatalf("DeleteItem returned error: %v", err)
	}
}

func TestShoppingLists(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/shopping-lists" {
			t.Fatalf("path = %s, want /api/v1/shopping-lists", r.URL.Path)
		}
		if got := r.URL.Query().Get("start"); got != "2025-02-01" {
			t.Fatalf("start = %q, want 2025-02-01", got)
		}
		if got := r.URL.Query().Get("end"); got != "2025-02-28" {
			t.Fatalf("end = %q, want 2025-02-28", got)
		}
		resp := []ShoppingList{
			{
				ID:        "list-1",
				ListDate:  "2025-02-01",
				Name:      "Weekly",
				CreatedAt: time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2025, 2, 1, 12, 0, 0, 0, time.UTC),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.ShoppingLists(context.Background(), "2025-02-01", "2025-02-28")
	if err != nil {
		t.Fatalf("ShoppingLists returned error: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("ShoppingLists length = %d, want 1", len(resp))
	}
}

func TestCreateShoppingList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/shopping-lists" {
			t.Fatalf("path = %s, want /api/v1/shopping-lists", r.URL.Path)
		}
		var payload struct {
			ListDate string  `json:"list_date"`
			Name     string  `json:"name"`
			Notes    *string `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.ListDate != "2025-02-03" {
			t.Fatalf("list_date = %q, want 2025-02-03", payload.ListDate)
		}
		if payload.Name != "Weekly" {
			t.Fatalf("name = %q, want Weekly", payload.Name)
		}
		resp := ShoppingList{
			ID:        "list-1",
			ListDate:  payload.ListDate,
			Name:      payload.Name,
			CreatedAt: time.Date(2025, 2, 3, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2025, 2, 3, 12, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.CreateShoppingList(context.Background(), "2025-02-03", "Weekly", nil)
	if err != nil {
		t.Fatalf("CreateShoppingList returned error: %v", err)
	}
	if resp.ID != "list-1" {
		t.Fatalf("ID = %q, want list-1", resp.ID)
	}
}

func TestShoppingList(t *testing.T) {
	t.Parallel()

	const shoppingListPath = "/api/v1/shopping-lists/list-1"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != shoppingListPath {
			t.Fatalf("path = %s, want %s", r.URL.Path, shoppingListPath)
		}
		resp := ShoppingListDetail{
			ID:       "list-1",
			ListDate: "2025-02-03",
			Name:     "Weekly",
			Items: []ShoppingListItem{
				{
					ID:   "item-1",
					Item: Item{ID: "item-1", Name: "milk"},
				},
			},
			CreatedAt: time.Date(2025, 2, 3, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2025, 2, 3, 12, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.ShoppingList(context.Background(), "list-1")
	if err != nil {
		t.Fatalf("ShoppingList returned error: %v", err)
	}
	if resp.ID != "list-1" {
		t.Fatalf("ID = %q, want list-1", resp.ID)
	}
}

func TestUpdateShoppingList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/api/v1/shopping-lists/list-1" {
			t.Fatalf("path = %s, want /api/v1/shopping-lists/list-1", r.URL.Path)
		}
		var payload struct {
			ListDate string  `json:"list_date"`
			Name     string  `json:"name"`
			Notes    *string `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		resp := ShoppingList{
			ID:        "list-1",
			ListDate:  payload.ListDate,
			Name:      payload.Name,
			CreatedAt: time.Date(2025, 2, 3, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2025, 2, 4, 12, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.UpdateShoppingList(context.Background(), "list-1", "2025-02-04", "Updated", nil)
	if err != nil {
		t.Fatalf("UpdateShoppingList returned error: %v", err)
	}
	if resp.Name != "Updated" {
		t.Fatalf("Name = %q, want Updated", resp.Name)
	}
}

func TestDeleteShoppingList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/v1/shopping-lists/list-1" {
			t.Fatalf("path = %s, want /api/v1/shopping-lists/list-1", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := api.DeleteShoppingList(context.Background(), "list-1"); err != nil {
		t.Fatalf("DeleteShoppingList returned error: %v", err)
	}
}

func TestShoppingListItems(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/shopping-lists/list-1/items" {
			t.Fatalf("path = %s, want /api/v1/shopping-lists/list-1/items", r.URL.Path)
		}
		resp := []ShoppingListItem{
			{
				ID:   "item-1",
				Item: Item{ID: "item-1", Name: "milk"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.ShoppingListItems(context.Background(), "list-1")
	if err != nil {
		t.Fatalf("ShoppingListItems returned error: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("ShoppingListItems length = %d, want 1", len(resp))
	}
}

func TestAddShoppingListItems(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/shopping-lists/list-1/items" {
			t.Fatalf("path = %s, want /api/v1/shopping-lists/list-1/items", r.URL.Path)
		}
		var payload struct {
			Items []ShoppingListItemInput `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Items) != 1 || payload.Items[0].ItemID != "item-1" {
			t.Fatalf("items = %+v, want item-1", payload.Items)
		}
		resp := []ShoppingListItem{
			{
				ID:   "list-item-1",
				Item: Item{ID: "item-1", Name: "milk"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = api.AddShoppingListItems(context.Background(), "list-1", []ShoppingListItemInput{{ItemID: "item-1"}})
	if err != nil {
		t.Fatalf("AddShoppingListItems returned error: %v", err)
	}
}

func TestAddShoppingListItemsFromRecipes(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/shopping-lists/list-1/items/from-recipes" {
			t.Fatalf("path = %s, want /api/v1/shopping-lists/list-1/items/from-recipes", r.URL.Path)
		}
		var payload struct {
			RecipeIDs []string `json:"recipe_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.RecipeIDs) != 2 {
			t.Fatalf("recipe_ids length = %d, want 2", len(payload.RecipeIDs))
		}
		resp := []ShoppingListItem{{ID: "list-item-1", Item: Item{ID: "item-1", Name: "milk"}}}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = api.AddShoppingListItemsFromRecipes(context.Background(), "list-1", []string{"recipe-1", "recipe-2"})
	if err != nil {
		t.Fatalf("AddShoppingListItemsFromRecipes returned error: %v", err)
	}
}

func TestAddShoppingListItemsFromMealPlan(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/shopping-lists/list-1/items/from-meal-plan" {
			t.Fatalf("path = %s, want /api/v1/shopping-lists/list-1/items/from-meal-plan", r.URL.Path)
		}
		var payload struct {
			Date string `json:"date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Date != "2025-02-10" {
			t.Fatalf("date = %q, want 2025-02-10", payload.Date)
		}
		resp := []ShoppingListItem{{ID: "list-item-1", Item: Item{ID: "item-1", Name: "milk"}}}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = api.AddShoppingListItemsFromMealPlan(context.Background(), "list-1", "2025-02-10")
	if err != nil {
		t.Fatalf("AddShoppingListItemsFromMealPlan returned error: %v", err)
	}
}

func TestUpdateShoppingListItemPurchase(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/api/v1/shopping-lists/list-1/items/item-1" {
			t.Fatalf("path = %s, want /api/v1/shopping-lists/list-1/items/item-1", r.URL.Path)
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
		resp := ShoppingListItem{
			ID:          "list-item-1",
			Item:        Item{ID: "item-1", Name: "milk"},
			IsPurchased: true,
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := api.UpdateShoppingListItemPurchase(context.Background(), "list-1", "item-1", true)
	if err != nil {
		t.Fatalf("UpdateShoppingListItemPurchase returned error: %v", err)
	}
	if !resp.IsPurchased {
		t.Fatalf("IsPurchased = false, want true")
	}
}

func TestDeleteShoppingListItem(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/v1/shopping-lists/list-1/items/item-1" {
			t.Fatalf("path = %s, want /api/v1/shopping-lists/list-1/items/item-1", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	api, err := New(server.URL, "pat_456", 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := api.DeleteShoppingListItem(context.Background(), "list-1", "item-1"); err != nil {
		t.Fatalf("DeleteShoppingListItem returned error: %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}
