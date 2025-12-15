package bootstrap

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/saiaj/cooking_app/backend/internal/auth/password"
	"github.com/saiaj/cooking_app/backend/internal/auth/users"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
)

// ErrAlreadyBootstrapped indicates a user already exists and bootstrap must not run.
var ErrAlreadyBootstrapped = errors.New("bootstrap refused: users table is not empty")

// FirstUserParams captures the inputs required to create the first user.
type FirstUserParams struct {
	Username    string
	Password    string
	DisplayName *string
}

// CreateFirstUser creates the initial user for a fresh database.
// It refuses to run when the `users` table is non-empty.
func CreateFirstUser(ctx context.Context, queries *sqlc.Queries, params FirstUserParams) (sqlc.User, error) {
	if ctx == nil {
		return sqlc.User{}, errors.New("context is required")
	}
	if queries == nil {
		return sqlc.User{}, errors.New("queries is required")
	}

	count, err := queries.CountUsers(ctx)
	if err != nil {
		return sqlc.User{}, err
	}
	if count != 0 {
		return sqlc.User{}, ErrAlreadyBootstrapped
	}

	hash, err := password.Hash(params.Password)
	if err != nil {
		return sqlc.User{}, err
	}

	repo, err := users.New(queries)
	if err != nil {
		return sqlc.User{}, err
	}

	id := uuid.New()
	return repo.Create(ctx, users.CreateParams{
		ID:           id,
		Username:     params.Username,
		PasswordHash: hash,
		DisplayName:  params.DisplayName,
		IsActive:     true,
		CreatedBy:    id,
		UpdatedBy:    id,
	})
}
