package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
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

func TestRecipes_Create(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)

	user, err := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	})
	if err != nil {
		t.Fatalf("bootstrap user: %v", err)
	}

	book, err := queries.CreateRecipeBook(ctx, sqlc.CreateRecipeBookParams{
		Name:      "Dinner",
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if err != nil {
		t.Fatalf("create recipe book: %v", err)
	}

	tagA, err := queries.CreateTag(ctx, sqlc.CreateTagParams{
		Name:      tagNameSoup,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if err != nil {
		t.Fatalf("create tag A: %v", err)
	}
	tagB, err := queries.CreateTag(ctx, sqlc.CreateTagParams{
		Name:      "Chicken",
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
	})
	if err != nil {
		t.Fatalf("create tag B: %v", err)
	}

	app, err := httpapi.New(ctx, logging.New("error"), config.Config{
		DatabaseURL:         postgres.DatabaseURL,
		LogLevel:            "error",
		SessionCookieName:   "cooking_app_session",
		SessionTTL:          24 * time.Hour,
		SessionCookieSecure: false,
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

	bookID := uuid.UUID(book.ID.Bytes).String()
	tagID1 := uuid.UUID(tagA.ID.Bytes).String()
	tagID2 := uuid.UUID(tagB.ID.Bytes).String()
	userID := uuid.UUID(user.ID.Bytes).String()

	body := fmt.Sprintf(`{
  "title":%q,
  "servings":4,
  "prep_time_minutes":15,
  "total_time_minutes":60,
  "source_url":"https://example.com",
  "notes":"Family favorite",
  "recipe_book_id":%q,
  "tag_ids":[%q,%q],
  "ingredients":[
    {"position":2,"quantity":0.5,"quantity_text":"1/2","unit":"lb","item":"carrot","prep":null,"notes":null,"original_text":"1/2 lb carrot"},
    {"position":1,"quantity":1.0,"quantity_text":"1","unit":"lb","item":"chicken","prep":null,"notes":null,"original_text":"1 lb chicken"}
  ],
  "steps":[
    {"step_number":2,"instruction":"Serve."},
    {"step_number":1,"instruction":"Boil the chicken."}
  ]
}`, recipeTitleChickenSoup, bookID, tagID1, tagID2)

	resp, err = client.Post(server.URL+"/api/v1/recipes", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post recipes: %v", err)
	}
	createBody := resp.Body
	t.Cleanup(func() {
		if closeErr := createBody.Close(); closeErr != nil {
			t.Errorf("close create body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status=%d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var created recipeDetailResponse
	if decodeErr := json.NewDecoder(createBody).Decode(&created); decodeErr != nil {
		t.Fatalf("decode create: %v", decodeErr)
	}

	if created.ID == "" {
		t.Fatalf("id missing")
	}
	if created.Title != recipeTitleChickenSoup {
		t.Fatalf("title=%q, want %q", created.Title, recipeTitleChickenSoup)
	}
	if created.RecipeBookID == nil || *created.RecipeBookID != bookID {
		t.Fatalf("recipe_book_id=%v, want %q", created.RecipeBookID, bookID)
	}
	if created.CreatedBy != userID || created.UpdatedBy != userID {
		t.Fatalf("created_by=%q updated_by=%q, want %q", created.CreatedBy, created.UpdatedBy, userID)
	}
	if created.CreatedAt == "" || created.UpdatedAt == "" {
		t.Fatalf("created_at=%q updated_at=%q, want both set", created.CreatedAt, created.UpdatedAt)
	}
	if created.DeletedAt != nil {
		t.Fatalf("deleted_at=%v, want nil", created.DeletedAt)
	}

	if len(created.Tags) != 2 {
		t.Fatalf("tags len=%d, want 2", len(created.Tags))
	}
	for _, tag := range created.Tags {
		if tag.ID == "" || tag.Name == "" {
			t.Fatalf("tag missing fields: %#v", tag)
		}
	}

	if len(created.Ingredients) != 2 {
		t.Fatalf("ingredients len=%d, want 2", len(created.Ingredients))
	}
	if created.Ingredients[0].Position != 1 || created.Ingredients[0].Item != "chicken" {
		t.Fatalf("ingredients[0]=%#v, want position 1 chicken", created.Ingredients[0])
	}
	if created.Ingredients[0].ID == "" || created.Ingredients[1].ID == "" {
		t.Fatalf("ingredient ids missing")
	}

	if len(created.Steps) != 2 {
		t.Fatalf("steps len=%d, want 2", len(created.Steps))
	}
	if created.Steps[0].StepNumber != 1 || created.Steps[0].Instruction != "Boil the chicken." {
		t.Fatalf("steps[0]=%#v, want step 1 boil", created.Steps[0])
	}
	if created.Steps[0].ID == "" || created.Steps[1].ID == "" {
		t.Fatalf("step ids missing")
	}
}

type recipeTagResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type recipeIngredientResponse struct {
	ID           string   `json:"id"`
	Position     int      `json:"position"`
	Quantity     *float64 `json:"quantity"`
	QuantityText *string  `json:"quantity_text"`
	Unit         *string  `json:"unit"`
	Item         string   `json:"item"`
	Prep         *string  `json:"prep"`
	Notes        *string  `json:"notes"`
	OriginalText *string  `json:"original_text"`
}

type recipeStepResponse struct {
	ID          string `json:"id"`
	StepNumber  int    `json:"step_number"`
	Instruction string `json:"instruction"`
}

type recipeDetailResponse struct {
	ID               string                     `json:"id"`
	Title            string                     `json:"title"`
	Servings         int                        `json:"servings"`
	PrepTimeMinutes  int                        `json:"prep_time_minutes"`
	TotalTimeMinutes int                        `json:"total_time_minutes"`
	SourceURL        *string                    `json:"source_url"`
	Notes            *string                    `json:"notes"`
	RecipeBookID     *string                    `json:"recipe_book_id"`
	Tags             []recipeTagResponse        `json:"tags"`
	Ingredients      []recipeIngredientResponse `json:"ingredients"`
	Steps            []recipeStepResponse       `json:"steps"`
	CreatedAt        string                     `json:"created_at"`
	CreatedBy        string                     `json:"created_by"`
	UpdatedAt        string                     `json:"updated_at"`
	UpdatedBy        string                     `json:"updated_by"`
	DeletedAt        *string                    `json:"deleted_at"`
}
