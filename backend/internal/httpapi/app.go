package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/saiaj/cooking_app/backend/internal/config"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
)

// App holds the HTTP router and shared dependencies for handlers.
type App struct {
	logger  *slog.Logger
	pool    *pgxpool.Pool
	queries *sqlc.Queries
	mux     http.Handler

	sessionCookieName   string
	sessionTTL          time.Duration
	sessionCookieSecure bool
	csrfCookieName      string
	csrfHeaderName      string

	loginLimiter       *rateLimiter
	tokenCreateLimiter *rateLimiter

	maxJSONBodyBytes int64
	strictJSON       bool
}

// New wires the API app (router + DB pool).
func New(ctx context.Context, logger *slog.Logger, cfg config.Config) (*App, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}

	app := &App{
		logger:              logger,
		pool:                pool,
		queries:             sqlc.New(pool),
		sessionCookieName:   cfg.SessionCookieName,
		sessionTTL:          cfg.SessionTTL,
		sessionCookieSecure: cfg.SessionCookieSecure,
		csrfCookieName:      cfg.SessionCookieName + "_csrf",
		csrfHeaderName:      "X-CSRF-Token",
		loginLimiter:        newRateLimiter(cfg.LoginRateLimitPerMin, cfg.LoginRateLimitBurst),
		tokenCreateLimiter:  newRateLimiter(cfg.TokenCreateRateLimitPerMin, cfg.TokenCreateRateLimitBurst),
		maxJSONBodyBytes:    cfg.MaxJSONBodyBytes,
		strictJSON:          cfg.StrictJSON,
	}
	app.mux = routes(app)

	return app, nil
}

// Handler returns the HTTP handler for the API service.
func (a *App) Handler() http.Handler {
	return a.mux
}

// Close releases owned resources (DB pool).
func (a *App) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}
