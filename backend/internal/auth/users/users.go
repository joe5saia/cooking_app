package users

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
)

// Repo provides user persistence helpers backed by sqlc queries.
type Repo struct {
	queries *sqlc.Queries
}

// New returns a users repository backed by sqlc queries.
func New(queries *sqlc.Queries) (*Repo, error) {
	if queries == nil {
		return nil, errors.New("queries is required")
	}
	return &Repo{queries: queries}, nil
}

// NormalizeUsername applies username normalization consistent with citext uniqueness.
func NormalizeUsername(username string) (string, error) {
	normalized := strings.TrimSpace(username)
	if normalized == "" {
		return "", errors.New("username is required")
	}
	return normalized, nil
}

// CreateParams captures inputs for creating a user.
type CreateParams struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	DisplayName  *string
	IsActive     bool
	CreatedBy    uuid.UUID
	UpdatedBy    uuid.UUID
}

// Create inserts a new user.
func (r *Repo) Create(ctx context.Context, params CreateParams) (sqlc.User, error) {
	if r == nil || r.queries == nil {
		return sqlc.User{}, errors.New("repo is required")
	}
	if ctx == nil {
		return sqlc.User{}, errors.New("context is required")
	}

	username, err := NormalizeUsername(params.Username)
	if err != nil {
		return sqlc.User{}, err
	}
	if params.PasswordHash == "" {
		return sqlc.User{}, errors.New("password hash is required")
	}

	return r.queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:           uuidToPG(params.ID),
		Username:     username,
		PasswordHash: params.PasswordHash,
		DisplayName:  textPtrToPG(params.DisplayName),
		IsActive:     params.IsActive,
		CreatedBy:    uuidToPG(params.CreatedBy),
		UpdatedBy:    uuidToPG(params.UpdatedBy),
	})
}

// GetByUsername fetches a user by username (case-insensitive due to citext).
func (r *Repo) GetByUsername(ctx context.Context, username string) (sqlc.User, error) {
	if r == nil || r.queries == nil {
		return sqlc.User{}, errors.New("repo is required")
	}
	if ctx == nil {
		return sqlc.User{}, errors.New("context is required")
	}

	normalized, err := NormalizeUsername(username)
	if err != nil {
		return sqlc.User{}, err
	}

	return r.queries.GetUserByUsername(ctx, normalized)
}

// GetByID fetches a user by id.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	if r == nil || r.queries == nil {
		return sqlc.User{}, errors.New("repo is required")
	}
	if ctx == nil {
		return sqlc.User{}, errors.New("context is required")
	}

	return r.queries.GetUserByID(ctx, uuidToPG(id))
}

// List returns all users ordered by creation time.
func (r *Repo) List(ctx context.Context) ([]sqlc.User, error) {
	if r == nil || r.queries == nil {
		return nil, errors.New("repo is required")
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}

	return r.queries.ListUsers(ctx)
}

// SetActive toggles a user's active status and updates audit fields.
func (r *Repo) SetActive(ctx context.Context, id uuid.UUID, isActive bool, updatedBy uuid.UUID) (sqlc.User, error) {
	if r == nil || r.queries == nil {
		return sqlc.User{}, errors.New("repo is required")
	}
	if ctx == nil {
		return sqlc.User{}, errors.New("context is required")
	}

	return r.queries.SetUserActive(ctx, sqlc.SetUserActiveParams{
		ID:        uuidToPG(id),
		IsActive:  isActive,
		UpdatedBy: uuidToPG(updatedBy),
	})
}

func uuidToPG(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func textPtrToPG(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}
