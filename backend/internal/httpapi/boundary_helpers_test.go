package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestParseUUIDParam(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		want := uuid.New()
		r := withURLParam(t, "/x/"+want.String(), "id", want.String())

		got, err := parseUUIDParam(r, "id")
		if err != nil {
			t.Fatalf("parseUUIDParam: %v", err)
		}
		if got != want {
			t.Fatalf("uuid=%s, want %s", got, want)
		}
	})

	t.Run("valid non-id param", func(t *testing.T) {
		t.Parallel()

		want := uuid.New()
		r := withURLParam(t, "/x/"+want.String(), "user_id", want.String())

		got, err := parseUUIDParam(r, "user_id")
		if err != nil {
			t.Fatalf("parseUUIDParam: %v", err)
		}
		if got != want {
			t.Fatalf("uuid=%s, want %s", got, want)
		}
	})

	t.Run("invalid returns validation error", func(t *testing.T) {
		t.Parallel()

		r := withURLParam(t, "/x/not-a-uuid", "id", "not-a-uuid")

		_, err := parseUUIDParam(r, "id")
		apiErr, ok := asAPIError(err)
		if !ok {
			t.Fatalf("expected apiError, got %T (%v)", err, err)
		}
		if apiErr.kind != apiErrorValidation {
			t.Fatalf("kind=%q, want %q", apiErr.kind, apiErrorValidation)
		}
	})
}

func TestIsPGUniqueViolation(t *testing.T) {
	t.Parallel()

	if !isPGUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatalf("expected true")
	}
	if isPGUniqueViolation(&pgconn.PgError{Code: "23503"}) {
		t.Fatalf("expected false")
	}
}

func withURLParam(t *testing.T, path, key, value string) *http.Request {
	t.Helper()

	r, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)

	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx))
}
