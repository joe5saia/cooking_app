package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/saiaj/cooking_app/backend/internal/bootstrap"
	"github.com/saiaj/cooking_app/backend/internal/config"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi"
	"github.com/saiaj/cooking_app/backend/internal/logging"
	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

func TestMealPlans_CRUD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)

	user, userErr := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	})
	if userErr != nil {
		t.Fatalf("bootstrap user: %v", userErr)
	}

	recipe, recipeErr := queries.CreateRecipe(ctx, sqlc.CreateRecipeParams{
		Title:            "Pasta",
		Servings:         2,
		PrepTimeMinutes:  5,
		TotalTimeMinutes: 15,
		CreatedBy:        user.ID,
		UpdatedBy:        user.ID,
	})
	if recipeErr != nil {
		t.Fatalf("create recipe: %v", recipeErr)
	}
	recipeID := uuid.UUID(recipe.ID.Bytes).String()

	app, err := httpapi.New(ctx, logging.New("error"), config.Config{
		DatabaseURL:         postgres.DatabaseURL,
		LogLevel:            "error",
		SessionCookieName:   "cooking_app_session",
		SessionTTL:          24 * time.Hour,
		SessionCookieSecure: false,
		MaxJSONBodyBytes:    2 << 20,
		StrictJSON:          true,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	t.Cleanup(app.Close)

	server := httptest.NewServer(app.Handler())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/api/v1/meal-plans?start=2025-01-01&end=2025-01-31")
	if err != nil {
		t.Fatalf("list unauth: %v", err)
	}
	body := resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth list status=%d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	csrf := loginAndGetCSRFToken(t, client, server.URL)

	req := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/meal-plans", `{"date":"2025-01-03","recipe_id":"`+recipeID+`"}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("create meal plan: %v", err)
	}
	createBody := resp.Body
	t.Cleanup(func() {
		if closeErr := createBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var created map[string]any
	if decodeErr := json.NewDecoder(createBody).Decode(&created); decodeErr != nil {
		t.Fatalf("decode create: %v", decodeErr)
	}
	if created["date"] != "2025-01-03" {
		t.Fatalf("date=%v, want 2025-01-03", created["date"])
	}

	req = newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/meal-plans", `{"date":"2025-01-03","recipe_id":"`+recipeID+`"}`)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("create meal plan dup: %v", err)
	}
	dupBody := resp.Body
	t.Cleanup(func() {
		if closeErr := dupBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("dup status=%d, want %d", resp.StatusCode, http.StatusConflict)
	}

	resp, err = client.Get(server.URL + "/api/v1/meal-plans?start=2025-01-01&end=2025-01-31")
	if err != nil {
		t.Fatalf("list meal plans: %v", err)
	}
	listBody := resp.Body
	t.Cleanup(func() {
		if closeErr := listBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var listed struct {
		Items []struct {
			Date   string `json:"date"`
			Recipe struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"recipe"`
		} `json:"items"`
	}
	if decodeErr := json.NewDecoder(listBody).Decode(&listed); decodeErr != nil {
		t.Fatalf("decode list: %v", decodeErr)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("listed len=%d, want 1", len(listed.Items))
	}
	if listed.Items[0].Recipe.Title != "Pasta" {
		t.Fatalf("title=%q, want Pasta", listed.Items[0].Recipe.Title)
	}

	req, err = http.NewRequest(http.MethodDelete, server.URL+"/api/v1/meal-plans/2025-01-03/"+recipeID, nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("delete meal plan: %v", err)
	}
	deleteBody := resp.Body
	t.Cleanup(func() {
		if closeErr := deleteBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	resp, err = client.Get(server.URL + "/api/v1/meal-plans?start=2025-01-01&end=2025-01-31")
	if err != nil {
		t.Fatalf("list meal plans after delete: %v", err)
	}
	afterBody := resp.Body
	t.Cleanup(func() {
		if closeErr := afterBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list after delete status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var afterListed struct {
		Items []any `json:"items"`
	}
	if decodeErr := json.NewDecoder(afterBody).Decode(&afterListed); decodeErr != nil {
		t.Fatalf("decode list after delete: %v", decodeErr)
	}
	if len(afterListed.Items) != 0 {
		t.Fatalf("listed len=%d, want 0", len(afterListed.Items))
	}
}
