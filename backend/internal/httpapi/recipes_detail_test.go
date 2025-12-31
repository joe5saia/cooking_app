package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/bootstrap"
	"github.com/saiaj/cooking_app/backend/internal/config"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi"
	"github.com/saiaj/cooking_app/backend/internal/logging"
	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

func TestRecipes_GetDetail(t *testing.T) {
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

	book, bookErr := queries.CreateRecipeBook(ctx, sqlc.CreateRecipeBookParams{
		Name:      "Dinner",
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if bookErr != nil {
		t.Fatalf("create recipe book: %v", bookErr)
	}

	tag, tagErr := queries.CreateTag(ctx, sqlc.CreateTagParams{
		Name:      tagNameSoup,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if tagErr != nil {
		t.Fatalf("create tag: %v", tagErr)
	}

	recipe, recipeErr := queries.CreateRecipe(ctx, sqlc.CreateRecipeParams{
		Title:            recipeTitleChickenSoup,
		Servings:         4,
		PrepTimeMinutes:  15,
		TotalTimeMinutes: 60,
		SourceUrl:        pgtype.Text{String: "https://example.com", Valid: true},
		Notes:            pgtype.Text{String: "Family favorite", Valid: true},
		RecipeBookID:     book.ID,
		CreatedBy:        user.ID,
		UpdatedBy:        user.ID,
	})
	if recipeErr != nil {
		t.Fatalf("create recipe: %v", recipeErr)
	}

	var qty1 pgtype.Numeric
	if scanErr := qty1.Scan(1.0); scanErr != nil {
		t.Fatalf("scan numeric: %v", scanErr)
	}
	var qty2 pgtype.Numeric
	if scanErr := qty2.Scan(0.5); scanErr != nil {
		t.Fatalf("scan numeric: %v", scanErr)
	}

	carrotItem, err := queries.CreateItem(ctx, sqlc.CreateItemParams{
		Name:      "carrot",
		StoreUrl:  pgtype.Text{},
		AisleID:   pgtype.UUID{},
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if err != nil {
		t.Fatalf("create item carrot: %v", err)
	}
	chickenItem, err := queries.CreateItem(ctx, sqlc.CreateItemParams{
		Name:      "chicken",
		StoreUrl:  pgtype.Text{},
		AisleID:   pgtype.UUID{},
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if err != nil {
		t.Fatalf("create item chicken: %v", err)
	}

	if err = queries.CreateRecipeIngredient(ctx, sqlc.CreateRecipeIngredientParams{
		RecipeID:     recipe.ID,
		Position:     2,
		Quantity:     qty2,
		QuantityText: pgtype.Text{String: "1/2", Valid: true},
		Unit:         pgtype.Text{String: "lb", Valid: true},
		ItemID:       carrotItem.ID,
		OriginalText: pgtype.Text{String: "1/2 lb carrot", Valid: true},
		CreatedBy:    user.ID,
		UpdatedBy:    user.ID,
	}); err != nil {
		t.Fatalf("create ingredient 2: %v", err)
	}
	if err = queries.CreateRecipeIngredient(ctx, sqlc.CreateRecipeIngredientParams{
		RecipeID:     recipe.ID,
		Position:     1,
		Quantity:     qty1,
		QuantityText: pgtype.Text{String: "1", Valid: true},
		Unit:         pgtype.Text{String: "lb", Valid: true},
		ItemID:       chickenItem.ID,
		OriginalText: pgtype.Text{String: "1 lb chicken", Valid: true},
		CreatedBy:    user.ID,
		UpdatedBy:    user.ID,
	}); err != nil {
		t.Fatalf("create ingredient 1: %v", err)
	}

	if err = queries.CreateRecipeStep(ctx, sqlc.CreateRecipeStepParams{
		RecipeID:    recipe.ID,
		StepNumber:  2,
		Instruction: "Serve.",
		CreatedBy:   user.ID,
		UpdatedBy:   user.ID,
	}); err != nil {
		t.Fatalf("create step 2: %v", err)
	}
	if err = queries.CreateRecipeStep(ctx, sqlc.CreateRecipeStepParams{
		RecipeID:    recipe.ID,
		StepNumber:  1,
		Instruction: "Boil the chicken.",
		CreatedBy:   user.ID,
		UpdatedBy:   user.ID,
	}); err != nil {
		t.Fatalf("create step 1: %v", err)
	}

	if err = queries.CreateRecipeTag(ctx, sqlc.CreateRecipeTagParams{
		RecipeID:  recipe.ID,
		TagID:     tag.ID,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	}); err != nil {
		t.Fatalf("create recipe tag: %v", err)
	}

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

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	resp, err := client.Post(server.URL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"username":"joe","password":"pw"}`))
	if err != nil {
		t.Fatalf("post login: %v", err)
	}
	loginBody := resp.Body
	t.Cleanup(func() {
		if closeErr := loginBody.Close(); closeErr != nil {
			t.Errorf("close login body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("login status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	recipeID := uuid.UUID(recipe.ID.Bytes).String()
	resp, err = client.Get(server.URL + "/api/v1/recipes/" + recipeID)
	if err != nil {
		t.Fatalf("get recipe: %v", err)
	}
	getBody := resp.Body
	t.Cleanup(func() {
		if closeErr := getBody.Close(); closeErr != nil {
			t.Errorf("close get body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status=%d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got recipeDetailResponse
	if decodeErr := json.NewDecoder(getBody).Decode(&got); decodeErr != nil {
		t.Fatalf("decode get: %v", decodeErr)
	}

	if got.ID != recipeID {
		t.Fatalf("id=%q, want %q", got.ID, recipeID)
	}
	if len(got.Tags) != 1 || got.Tags[0].Name != tagNameSoup {
		t.Fatalf("tags=%v, want %s", got.Tags, tagNameSoup)
	}
	if len(got.Ingredients) != 2 || got.Ingredients[0].Position != 1 || got.Ingredients[0].Item.Name != "chicken" {
		t.Fatalf("ingredients=%v, want ordered by position", got.Ingredients)
	}
	if len(got.Steps) != 2 || got.Steps[0].StepNumber != 1 || got.Steps[0].Instruction != "Boil the chicken." {
		t.Fatalf("steps=%v, want ordered by step_number", got.Steps)
	}
}

func TestRecipes_GetDetail_InvalidID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)
	if _, err := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	}); err != nil {
		t.Fatalf("bootstrap user: %v", err)
	}

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

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	resp, err := client.Post(server.URL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"username":"joe","password":"pw"}`))
	if err != nil {
		t.Fatalf("post login: %v", err)
	}
	loginBody := resp.Body
	t.Cleanup(func() {
		if closeErr := loginBody.Close(); closeErr != nil {
			t.Errorf("close login body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("login status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	resp, err = client.Get(server.URL + "/api/v1/recipes/not-a-uuid")
	if err != nil {
		t.Fatalf("get recipe: %v", err)
	}
	body := resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRecipes_GetDetail_NotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)
	if _, err := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	}); err != nil {
		t.Fatalf("bootstrap user: %v", err)
	}

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

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	resp, err := client.Post(server.URL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"username":"joe","password":"pw"}`))
	if err != nil {
		t.Fatalf("post login: %v", err)
	}
	loginBody := resp.Body
	t.Cleanup(func() {
		if closeErr := loginBody.Close(); closeErr != nil {
			t.Errorf("close login body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("login status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	resp, err = client.Get(server.URL + "/api/v1/recipes/" + uuid.NewString())
	if err != nil {
		t.Fatalf("get recipe: %v", err)
	}
	body := resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
