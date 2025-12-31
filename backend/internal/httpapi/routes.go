package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

func routes(app *App) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(app.requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if _, err := w.Write([]byte("ok\n")); err != nil {
			app.logger.Warn("write failed", "err", err, "path", "/healthz")
		}
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			app.writeError(w, r, errNotFound())
		})

		r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			app.writeError(w, r, errMethodNotAllowed())
		})

		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			if err := response.WriteJSON(w, http.StatusOK, struct {
				OK bool `json:"ok"`
			}{OK: true}); err != nil {
				app.logger.Warn("write failed", "err", err, "path", "/api/v1/healthz")
			}
		})

		r.Route("/auth", func(r chi.Router) {
			r.With(app.loginRateLimitMiddleware).Post("/login", app.handle(app.handleLogin))
			r.With(app.authMiddleware).Post("/logout", app.handle(app.handleLogout))
			r.With(app.authMiddleware).Get("/me", app.handle(app.handleMe))
		})

		r.Route("/tokens", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handle(app.handleTokensList))
			r.Post("/", app.handle(app.handleTokensCreate))
			r.Delete("/{id}", app.handle(app.handleTokensDelete))
		})

		r.Route("/users", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handle(app.handleUsersList))
			r.Post("/", app.handle(app.handleUsersCreate))
			r.Put("/{id}/deactivate", app.handle(app.handleUsersDeactivate))
		})

		r.Route("/tags", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handle(app.handleTagsList))
			r.Post("/", app.handle(app.handleTagsCreate))
			r.Put("/{id}", app.handle(app.handleTagsUpdate))
			r.Delete("/{id}", app.handle(app.handleTagsDelete))
		})

		r.Route("/aisles", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handle(app.handleAislesList))
			r.Get("/{id}", app.handle(app.handleAislesGet))
			r.Post("/", app.handle(app.handleAislesCreate))
			r.Put("/{id}", app.handle(app.handleAislesUpdate))
			r.Delete("/{id}", app.handle(app.handleAislesDelete))
		})

		r.Route("/items", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handle(app.handleItemsList))
			r.Get("/{id}", app.handle(app.handleItemsGet))
			r.Post("/", app.handle(app.handleItemsCreate))
			r.Put("/{id}", app.handle(app.handleItemsUpdate))
			r.Delete("/{id}", app.handle(app.handleItemsDelete))
		})

		r.Route("/shopping-lists", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handle(app.handleShoppingListsList))
			r.Post("/", app.handle(app.handleShoppingListsCreate))
			r.Get("/{id}", app.handle(app.handleShoppingListsGet))
			r.Put("/{id}", app.handle(app.handleShoppingListsUpdate))
			r.Delete("/{id}", app.handle(app.handleShoppingListsDelete))
			r.Get("/{id}/items", app.handle(app.handleShoppingListItemsList))
			r.Post("/{id}/items", app.handle(app.handleShoppingListItemsAdd))
			r.Post("/{id}/items/from-recipes", app.handle(app.handleShoppingListItemsAddFromRecipes))
			r.Post("/{id}/items/from-meal-plan", app.handle(app.handleShoppingListItemsAddFromMealPlan))
			r.Patch("/{id}/items/{item_id}", app.handle(app.handleShoppingListItemsUpdate))
			r.Delete("/{id}/items/{item_id}", app.handle(app.handleShoppingListItemsDelete))
		})

		r.Route("/recipe-books", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handle(app.handleRecipeBooksList))
			r.Post("/", app.handle(app.handleRecipeBooksCreate))
			r.Put("/{id}", app.handle(app.handleRecipeBooksUpdate))
			r.Delete("/{id}", app.handle(app.handleRecipeBooksDelete))
		})

		r.Route("/recipes", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handle(app.handleRecipesList))
			r.Get("/{id}", app.handle(app.handleRecipesGet))
			r.Post("/", app.handle(app.handleRecipesCreate))
			r.Put("/{id}", app.handle(app.handleRecipesUpdate))
			r.Delete("/{id}", app.handle(app.handleRecipesDelete))
			r.Put("/{id}/restore", app.handle(app.handleRecipesRestore))
		})

		r.Route("/meal-plans", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handle(app.handleMealPlansList))
			r.Post("/", app.handle(app.handleMealPlansCreate))
			r.Delete("/{date}/{recipe_id}", app.handle(app.handleMealPlansDelete))
		})
	})

	return r
}
