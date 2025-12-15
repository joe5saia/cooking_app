package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/saiaj/cooking_app/backend/internal/auth/password"
	"github.com/saiaj/cooking_app/backend/internal/auth/users"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

func TestRepo_CreateAndGetByUsername(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)

	repo, err := users.New(queries)
	if err != nil {
		t.Fatalf("New repo: %v", err)
	}

	pwHash, err := password.Hash("secret-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	id := uuid.New()
	createdBy := id

	created, err := repo.Create(ctx, users.CreateParams{
		ID:           id,
		Username:     "  Alice  ",
		PasswordHash: pwHash,
		DisplayName:  nil,
		IsActive:     true,
		CreatedBy:    createdBy,
		UpdatedBy:    createdBy,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}

	if got.ID != created.ID {
		t.Fatalf("id mismatch")
	}
	if got.Username != "Alice" {
		t.Fatalf("username=%q, want %q", got.Username, "Alice")
	}
	if got.IsActive != true {
		t.Fatalf("is_active=%t, want true", got.IsActive)
	}
}
