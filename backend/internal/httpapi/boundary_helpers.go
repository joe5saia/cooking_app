package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// parseUUIDParam parses a required UUID path parameter and returns a typed
// error suitable for returning from HTTP handlers.
func parseUUIDParam(r *http.Request, paramName string) (uuid.UUID, error) {
	raw := chi.URLParam(r, paramName)
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, errValidationField(paramName, "invalid id")
	}
	return parsed, nil
}

// isPGUniqueViolation returns true when err indicates a Postgres uniqueness
// constraint violation (SQLSTATE 23505).
func isPGUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
