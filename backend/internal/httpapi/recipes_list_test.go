package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestRecipes_List(t *testing.T) {
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

	bookA, bookAErr := queries.CreateRecipeBook(ctx, sqlc.CreateRecipeBookParams{
		Name:      "Dinner",
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if bookAErr != nil {
		t.Fatalf("create book A: %v", bookAErr)
	}
	bookB, bookBErr := queries.CreateRecipeBook(ctx, sqlc.CreateRecipeBookParams{
		Name:      "Lunch",
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if bookBErr != nil {
		t.Fatalf("create book B: %v", bookBErr)
	}

	tagSoup, tagSoupErr := queries.CreateTag(ctx, sqlc.CreateTagParams{
		Name:      tagNameSoup,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if tagSoupErr != nil {
		t.Fatalf("create tag soup: %v", tagSoupErr)
	}

	tagBeef, tagBeefErr := queries.CreateTag(ctx, sqlc.CreateTagParams{
		Name:      "Beef",
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if tagBeefErr != nil {
		t.Fatalf("create tag beef: %v", tagBeefErr)
	}

	recipeChicken, chickenErr := queries.CreateRecipe(ctx, sqlc.CreateRecipeParams{
		Title:            recipeTitleChickenSoup,
		Servings:         4,
		PrepTimeMinutes:  15,
		TotalTimeMinutes: 60,
		RecipeBookID:     bookA.ID,
		CreatedBy:        user.ID,
		UpdatedBy:        user.ID,
	})
	if chickenErr != nil {
		t.Fatalf("create chicken: %v", chickenErr)
	}

	recipeBeef, beefErr := queries.CreateRecipe(ctx, sqlc.CreateRecipeParams{
		Title:            recipeTitleBeefStew,
		Servings:         6,
		PrepTimeMinutes:  20,
		TotalTimeMinutes: 90,
		RecipeBookID:     bookB.ID,
		CreatedBy:        user.ID,
		UpdatedBy:        user.ID,
	})
	if beefErr != nil {
		t.Fatalf("create beef: %v", beefErr)
	}

	recipeDeleted, deletedErr := queries.CreateRecipe(ctx, sqlc.CreateRecipeParams{
		Title:            "Deleted Recipe",
		Servings:         1,
		PrepTimeMinutes:  0,
		TotalTimeMinutes: 0,
		CreatedBy:        user.ID,
		UpdatedBy:        user.ID,
	})
	if deletedErr != nil {
		t.Fatalf("create deleted recipe: %v", deletedErr)
	}

	if _, execErr := pool.Exec(ctx, "UPDATE recipes SET updated_at=$1, updated_by=$2 WHERE id=$3", time.Date(2025, 12, 13, 10, 0, 0, 0, time.UTC), user.ID, recipeChicken.ID); execErr != nil {
		t.Fatalf("update chicken updated_at: %v", execErr)
	}
	if _, execErr := pool.Exec(ctx, "UPDATE recipes SET updated_at=$1, updated_by=$2 WHERE id=$3", time.Date(2025, 12, 13, 11, 0, 0, 0, time.UTC), user.ID, recipeBeef.ID); execErr != nil {
		t.Fatalf("update beef updated_at: %v", execErr)
	}
	if _, execErr := pool.Exec(ctx, "UPDATE recipes SET deleted_at=$1, updated_by=$2 WHERE id=$3", time.Date(2025, 12, 13, 12, 0, 0, 0, time.UTC), user.ID, recipeDeleted.ID); execErr != nil {
		t.Fatalf("update deleted_at: %v", execErr)
	}

	if tagErr := queries.CreateRecipeTag(ctx, sqlc.CreateRecipeTagParams{
		RecipeID:  recipeChicken.ID,
		TagID:     tagSoup.ID,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	}); tagErr != nil {
		t.Fatalf("create chicken tag: %v", tagErr)
	}
	if tagErr := queries.CreateRecipeTag(ctx, sqlc.CreateRecipeTagParams{
		RecipeID:  recipeBeef.ID,
		TagID:     tagBeef.ID,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	}); tagErr != nil {
		t.Fatalf("create beef tag: %v", tagErr)
	}

	app, appErr := httpapi.New(ctx, logging.New("error"), config.Config{
		DatabaseURL:         postgres.DatabaseURL,
		LogLevel:            "error",
		SessionCookieName:   "cooking_app_session",
		SessionTTL:          24 * time.Hour,
		SessionCookieSecure: false,
		MaxJSONBodyBytes:    2 << 20,
		StrictJSON:          true,
	})
	if appErr != nil {
		t.Fatalf("new app: %v", appErr)
	}
	t.Cleanup(app.Close)

	server := httptest.NewServer(app.Handler())
	t.Cleanup(server.Close)

	jar, jarErr := cookiejar.New(nil)
	if jarErr != nil {
		t.Fatalf("cookie jar: %v", jarErr)
	}
	client := &http.Client{Jar: jar}

	loginResp, loginErr := client.Post(server.URL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"username":"joe","password":"pw"}`))
	if loginErr != nil {
		t.Fatalf("post login: %v", loginErr)
	}
	loginBody := loginResp.Body
	t.Cleanup(func() {
		if closeErr := loginBody.Close(); closeErr != nil {
			t.Errorf("close login body: %v", closeErr)
		}
	})
	if loginResp.StatusCode != http.StatusNoContent {
		t.Fatalf("login status=%d, want %d", loginResp.StatusCode, http.StatusNoContent)
	}

	t.Run("default excludes deleted + ordered by updated_at", func(t *testing.T) {
		out := getRecipesList(t, client, server.URL, url.Values{})
		if len(out.Items) != 2 {
			t.Fatalf("items len=%d, want 2", len(out.Items))
		}
		if out.Items[0].Title != recipeTitleBeefStew || out.Items[1].Title != recipeTitleChickenSoup {
			t.Fatalf("items=%v, want [Beef Stew, Chicken Soup]", []string{out.Items[0].Title, out.Items[1].Title})
		}
		if out.NextCursor != nil {
			t.Fatalf("next_cursor=%v, want nil", *out.NextCursor)
		}
	})

	t.Run("include_deleted includes deleted recipe", func(t *testing.T) {
		q := url.Values{}
		q.Set("include_deleted", "true")
		out := getRecipesList(t, client, server.URL, q)
		if len(out.Items) != 3 {
			t.Fatalf("items len=%d, want 3", len(out.Items))
		}
	})

	t.Run("q filters by title", func(t *testing.T) {
		q := url.Values{}
		q.Set("q", "chicken")
		out := getRecipesList(t, client, server.URL, q)
		if len(out.Items) != 1 || out.Items[0].Title != recipeTitleChickenSoup {
			t.Fatalf("items=%v, want %s", out.Items, recipeTitleChickenSoup)
		}
	})

	t.Run("book_id filters by recipe book", func(t *testing.T) {
		q := url.Values{}
		q.Set("book_id", uuid.UUID(bookB.ID.Bytes).String())
		out := getRecipesList(t, client, server.URL, q)
		if len(out.Items) != 1 || out.Items[0].Title != recipeTitleBeefStew {
			t.Fatalf("items=%v, want %s", out.Items, recipeTitleBeefStew)
		}
	})

	t.Run("tag_id filters by tag", func(t *testing.T) {
		q := url.Values{}
		q.Set("tag_id", uuid.UUID(tagSoup.ID.Bytes).String())
		out := getRecipesList(t, client, server.URL, q)
		if len(out.Items) != 1 || out.Items[0].Title != recipeTitleChickenSoup {
			t.Fatalf("items=%v, want %s", out.Items, recipeTitleChickenSoup)
		}
		if len(out.Items[0].Tags) != 1 || out.Items[0].Tags[0].Name != tagNameSoup {
			t.Fatalf("tags=%v, want %s", out.Items[0].Tags, tagNameSoup)
		}
	})

	t.Run("cursor pagination", func(t *testing.T) {
		q := url.Values{}
		q.Set("limit", "1")
		page1 := getRecipesList(t, client, server.URL, q)
		if len(page1.Items) != 1 || page1.Items[0].Title != recipeTitleBeefStew {
			t.Fatalf("page1 items=%v, want %s", page1.Items, recipeTitleBeefStew)
		}
		if page1.NextCursor == nil || *page1.NextCursor == "" {
			t.Fatalf("page1 next_cursor missing")
		}

		q2 := url.Values{}
		q2.Set("limit", "1")
		q2.Set("cursor", *page1.NextCursor)
		page2 := getRecipesList(t, client, server.URL, q2)
		if len(page2.Items) != 1 || page2.Items[0].Title != recipeTitleChickenSoup {
			t.Fatalf("page2 items=%v, want %s", page2.Items, recipeTitleChickenSoup)
		}
		if page2.NextCursor != nil {
			t.Fatalf("page2 next_cursor=%v, want nil", *page2.NextCursor)
		}
	})
}

func getRecipesList(t *testing.T, client *http.Client, baseURL string, q url.Values) recipesListResponse {
	t.Helper()

	u := baseURL + "/api/v1/recipes"
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	resp, err := client.Get(u)
	if err != nil {
		t.Fatalf("get list: %v", err)
	}
	body := resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out recipesListResponse
	if decodeErr := json.NewDecoder(body).Decode(&out); decodeErr != nil {
		t.Fatalf("decode: %v", decodeErr)
	}
	return out
}

type recipeListTagResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type recipeListItemResponse struct {
	ID        string                  `json:"id"`
	Title     string                  `json:"title"`
	Tags      []recipeListTagResponse `json:"tags"`
	UpdatedAt string                  `json:"updated_at"`
}

type recipesListResponse struct {
	Items      []recipeListItemResponse `json:"items"`
	NextCursor *string                  `json:"next_cursor"`
}
