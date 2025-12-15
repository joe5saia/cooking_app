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
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			if err := response.WriteProblem(w, http.StatusNotFound, "not_found", "not found", nil); err != nil {
				app.logger.Warn("write failed", "err", err, "path", "/api/v1/*")
			}
		})

		r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
			if err := response.WriteProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil); err != nil {
				app.logger.Warn("write failed", "err", err, "path", "/api/v1/*")
			}
		})

		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			if err := response.WriteJSON(w, http.StatusOK, struct {
				OK bool `json:"ok"`
			}{OK: true}); err != nil {
				app.logger.Warn("write failed", "err", err, "path", "/api/v1/healthz")
			}
		})

		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", app.handleLogin)
			r.Post("/logout", app.handleLogout)
			r.Get("/me", app.handleMe)
		})

		r.Route("/tokens", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handleTokensList)
			r.Post("/", app.handleTokensCreate)
			r.Delete("/{id}", app.handleTokensDelete)
		})

		r.Route("/users", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handleUsersList)
			r.Post("/", app.handleUsersCreate)
			r.Put("/{id}/deactivate", app.handleUsersDeactivate)
		})

		r.Route("/tags", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handleTagsList)
			r.Post("/", app.handleTagsCreate)
			r.Put("/{id}", app.handleTagsUpdate)
			r.Delete("/{id}", app.handleTagsDelete)
		})

		r.Route("/recipe-books", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handleRecipeBooksList)
			r.Post("/", app.handleRecipeBooksCreate)
			r.Put("/{id}", app.handleRecipeBooksUpdate)
			r.Delete("/{id}", app.handleRecipeBooksDelete)
		})

		r.Route("/recipes", func(r chi.Router) {
			r.Use(app.authMiddleware)
			r.Get("/", app.handleRecipesList)
			r.Get("/{id}", app.handleRecipesGet)
			r.Post("/", app.handleRecipesCreate)
			r.Put("/{id}", app.handleRecipesUpdate)
			r.Delete("/{id}", app.handleRecipesDelete)
			r.Put("/{id}/restore", app.handleRecipesRestore)
		})
	})

	return r
}
