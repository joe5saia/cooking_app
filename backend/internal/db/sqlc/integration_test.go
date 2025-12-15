package sqlc_test

import (
	"context"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

func TestQueries_Ping(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)

	got, err := queries.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping error: %v", err)
	}
	if got != 1 {
		t.Fatalf("Ping=%d, want 1", got)
	}
}
