package bootstrap_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/bootstrap"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

func TestCreateFirstUser_EmptyAndNonEmpty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)

	u1, err := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "admin",
		Password:    "super-secret-password",
		DisplayName: nil,
	})
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	if u1.CreatedBy != u1.ID || u1.UpdatedBy != u1.ID {
		t.Fatalf("expected created_by and updated_by to self-reference id")
	}

	if _, err := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "admin2",
		Password:    "another-password",
		DisplayName: nil,
	}); !errors.Is(err, bootstrap.ErrAlreadyBootstrapped) {
		t.Fatalf("expected ErrAlreadyBootstrapped, got %v", err)
	}
}
