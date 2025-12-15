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

func TestRecipeBooks_CRUD_DeleteConflictWhenRecipesExist(t *testing.T) {
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

	resp, err := http.Get(server.URL + "/api/v1/recipe-books")
	if err != nil {
		t.Fatalf("get recipe books unauth: %v", err)
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

	resp, err = client.Post(server.URL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"username":"joe","password":"pw"}`))
	if err != nil {
		t.Fatalf("post login: %v", err)
	}
	loginBody := resp.Body
	t.Cleanup(func() {
		if closeErr := loginBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("login status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	resp, err = client.Post(server.URL+"/api/v1/recipe-books", "application/json", strings.NewReader(`{"name":"Main"}`))
	if err != nil {
		t.Fatalf("post recipe books: %v", err)
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
	id, ok := created["id"].(string)
	if !ok || id == "" {
		t.Fatalf("id missing or not string: %v", created["id"])
	}

	req, err := http.NewRequest(http.MethodPut, server.URL+"/api/v1/recipe-books/"+id, strings.NewReader(`{"name":"Primary"}`))
	if err != nil {
		t.Fatalf("new update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("update recipe book: %v", err)
	}
	updateBody := resp.Body
	t.Cleanup(func() {
		if closeErr := updateBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status=%d, want %d", resp.StatusCode, http.StatusOK)
	}

	req, err = http.NewRequest(http.MethodDelete, server.URL+"/api/v1/recipe-books/"+id, nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("delete recipe book: %v", err)
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

	resp, err = client.Post(server.URL+"/api/v1/recipe-books", "application/json", strings.NewReader(`{"name":"WithRecipes"}`))
	if err != nil {
		t.Fatalf("post recipe books 2: %v", err)
	}
	createBody2 := resp.Body
	t.Cleanup(func() {
		if closeErr := createBody2.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create2 status=%d, want %d", resp.StatusCode, http.StatusOK)
	}
	created = nil
	if decodeErr := json.NewDecoder(createBody2).Decode(&created); decodeErr != nil {
		t.Fatalf("decode create2: %v", decodeErr)
	}
	id2, ok := created["id"].(string)
	if !ok || id2 == "" {
		t.Fatalf("id2 missing or not string: %v", created["id"])
	}

	// Create a recipe referencing the recipe book directly to validate delete conflict behavior.
	bookID, err := uuid.Parse(id2)
	if err != nil {
		t.Fatalf("parse book id: %v", err)
	}
	_, err = pool.Exec(ctx, `
INSERT INTO recipes (
  title,
  servings,
  prep_time_minutes,
  total_time_minutes,
  source_url,
  notes,
  recipe_book_id,
  deleted_at,
  created_by,
  updated_by
) VALUES (
  'Soup',
  1,
  0,
  0,
  NULL,
  NULL,
  $1,
  NULL,
  $2,
  $2
)
`, pgtype.UUID{Bytes: bookID, Valid: true}, user.ID)
	if err != nil {
		t.Fatalf("insert recipe: %v", err)
	}

	req, err = http.NewRequest(http.MethodDelete, server.URL+"/api/v1/recipe-books/"+id2, nil)
	if err != nil {
		t.Fatalf("new delete2 request: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("delete2 recipe book: %v", err)
	}
	conflictBody := resp.Body
	t.Cleanup(func() {
		if closeErr := conflictBody.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("delete2 status=%d, want %d", resp.StatusCode, http.StatusConflict)
	}
}
