package httpapi_test

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
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

func TestRecipes_DeleteRestore(t *testing.T) {
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
		Title:            recipeTitleChickenSoup,
		Servings:         4,
		PrepTimeMinutes:  0,
		TotalTimeMinutes: 0,
		CreatedBy:        user.ID,
		UpdatedBy:        user.ID,
	})
	if recipeErr != nil {
		t.Fatalf("create recipe: %v", recipeErr)
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

	recipeID := uuid.UUID(recipe.ID.Bytes).String()

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/api/v1/recipes/"+recipeID, nil)
	if reqErr != nil {
		t.Fatalf("new request: %v", reqErr)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	resp, doErr := client.Do(req)
	if doErr != nil {
		t.Fatalf("delete recipe: %v", doErr)
	}
	body := resp.Body
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close delete body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	out := getRecipesList(t, client, server.URL, url.Values{})
	if len(out.Items) != 0 {
		t.Fatalf("items len=%d, want 0 after delete", len(out.Items))
	}

	req, reqErr = http.NewRequestWithContext(ctx, http.MethodPut, server.URL+"/api/v1/recipes/"+recipeID+"/restore", nil)
	if reqErr != nil {
		t.Fatalf("new restore request: %v", reqErr)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	resp, doErr = client.Do(req)
	if doErr != nil {
		t.Fatalf("restore recipe: %v", doErr)
	}
	restoreBody := resp.Body
	t.Cleanup(func() {
		if closeErr := restoreBody.Close(); closeErr != nil {
			t.Errorf("close restore body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("restore status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	out = getRecipesList(t, client, server.URL, url.Values{})
	if len(out.Items) != 1 || out.Items[0].ID != recipeID {
		t.Fatalf("items=%v, want restored recipe", out.Items)
	}
}

func TestRecipes_DeleteRestore_Errors(t *testing.T) {
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

	csrf := loginAndGetCSRFToken(t, client, server.URL)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/api/v1/recipes/not-a-uuid", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	deleteBody := resp.Body
	t.Cleanup(func() {
		if closeErr := deleteBody.Close(); closeErr != nil {
			t.Errorf("close delete body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("delete status=%d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodPut, server.URL+"/api/v1/recipes/not-a-uuid/restore", nil)
	if err != nil {
		t.Fatalf("new restore request: %v", err)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("restore request: %v", err)
	}
	restoreBody := resp.Body
	t.Cleanup(func() {
		if closeErr := restoreBody.Close(); closeErr != nil {
			t.Errorf("close restore body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("restore status=%d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	missingID := uuid.NewString()
	req, err = http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/api/v1/recipes/"+missingID, nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	missingDeleteBody := resp.Body
	t.Cleanup(func() {
		if closeErr := missingDeleteBody.Close(); closeErr != nil {
			t.Errorf("close delete body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("delete missing status=%d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodPut, server.URL+"/api/v1/recipes/"+missingID+"/restore", nil)
	if err != nil {
		t.Fatalf("new restore request: %v", err)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("restore request: %v", err)
	}
	missingRestoreBody := resp.Body
	t.Cleanup(func() {
		if closeErr := missingRestoreBody.Close(); closeErr != nil {
			t.Errorf("close restore body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("restore missing status=%d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
