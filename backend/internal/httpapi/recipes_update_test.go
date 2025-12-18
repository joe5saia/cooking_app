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

func TestRecipes_Update(t *testing.T) {
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

	csrf := loginAndGetCSRFToken(t, client, server.URL)

	bookAID := uuid.UUID(bookA.ID.Bytes).String()
	bookBID := uuid.UUID(bookB.ID.Bytes).String()
	tagSoupID := uuid.UUID(tagSoup.ID.Bytes).String()
	tagBeefID := uuid.UUID(tagBeef.ID.Bytes).String()

	createBody := fmt.Sprintf(`{
  "title":%q,
  "servings":4,
  "prep_time_minutes":0,
  "total_time_minutes":0,
  "source_url":null,
  "notes":null,
  "recipe_book_id":%q,
  "tag_ids":[%q],
  "ingredients":[{"position":1,"quantity":1.0,"quantity_text":"1","unit":"lb","item":"chicken","prep":null,"notes":null,"original_text":null}],
  "steps":[{"step_number":1,"instruction":"Boil."}]
}`, recipeTitleChickenSoup, bookAID, tagSoupID)

	req := newJSONRequest(t, http.MethodPost, server.URL+"/api/v1/recipes", createBody)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post create: %v", err)
	}
	body := resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close create body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status=%d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created recipeDetailResponse
	if decodeErr := json.NewDecoder(body).Decode(&created); decodeErr != nil {
		t.Fatalf("decode create: %v", decodeErr)
	}
	if created.ID == "" {
		t.Fatalf("created id missing")
	}
	prevUpdatedAt := created.UpdatedAt

	updateBody := fmt.Sprintf(`{
  "title":%q,
  "servings":6,
  "prep_time_minutes":10,
  "total_time_minutes":30,
  "source_url":"https://example.com",
  "notes":"Updated notes",
  "recipe_book_id":%q,
  "tag_ids":[%q],
  "ingredients":[{"position":1,"quantity":2.0,"quantity_text":"2","unit":"tbsp","item":"salt","prep":null,"notes":null,"original_text":null}],
  "steps":[{"step_number":1,"instruction":"Mix."},{"step_number":2,"instruction":"Serve."}]
}`, recipeTitleBeefStew, bookBID, tagBeefID)

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPut, server.URL+"/api/v1/recipes/"+created.ID, strings.NewReader(updateBody))
	if reqErr != nil {
		t.Fatalf("new update req: %v", reqErr)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("do update: %v", err)
	}
	updateRespBody := resp.Body
	t.Cleanup(func() {
		if closeErr := updateRespBody.Close(); closeErr != nil {
			t.Errorf("close update body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	var updated recipeDetailResponse
	if decodeErr := json.NewDecoder(updateRespBody).Decode(&updated); decodeErr != nil {
		t.Fatalf("decode update: %v", decodeErr)
	}
	if updated.Title != recipeTitleBeefStew {
		t.Fatalf("title=%q, want %q", updated.Title, recipeTitleBeefStew)
	}
	if updated.RecipeBookID == nil || *updated.RecipeBookID != bookBID {
		t.Fatalf("recipe_book_id=%v, want %q", updated.RecipeBookID, bookBID)
	}
	if len(updated.Tags) != 1 || updated.Tags[0].ID != tagBeefID {
		t.Fatalf("tags=%v, want beef tag", updated.Tags)
	}
	if len(updated.Ingredients) != 1 || updated.Ingredients[0].Item != "salt" {
		t.Fatalf("ingredients=%v, want salt only", updated.Ingredients)
	}
	if len(updated.Steps) != 2 || updated.Steps[1].StepNumber != 2 {
		t.Fatalf("steps=%v, want 2 steps", updated.Steps)
	}
	if updated.UpdatedAt == "" || updated.UpdatedAt == prevUpdatedAt {
		t.Fatalf("updated_at=%q prev=%q, want changed", updated.UpdatedAt, prevUpdatedAt)
	}

	deleteReq, reqErr := http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/api/v1/recipes/"+created.ID, nil)
	if reqErr != nil {
		t.Fatalf("new delete req: %v", reqErr)
	}
	deleteReq.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(deleteReq)
	if err != nil {
		t.Fatalf("do delete: %v", err)
	}
	deleteBodyResp := resp.Body
	t.Cleanup(func() {
		if closeErr := deleteBodyResp.Close(); closeErr != nil {
			t.Errorf("close delete body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	req, reqErr = http.NewRequestWithContext(ctx, http.MethodPut, server.URL+"/api/v1/recipes/"+created.ID, strings.NewReader(updateBody))
	if reqErr != nil {
		t.Fatalf("new update req: %v", reqErr)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("do update deleted: %v", err)
	}
	conflictBody := resp.Body
	t.Cleanup(func() {
		if closeErr := conflictBody.Close(); closeErr != nil {
			t.Errorf("close conflict body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("update deleted status=%d, want %d", resp.StatusCode, http.StatusConflict)
	}
}
