-- name: CountUsers :one
SELECT COUNT(*)::int AS count FROM users;

-- name: CreateUser :one
INSERT INTO users (
  id,
  username,
  password_hash,
  display_name,
  is_active,
  created_by,
  updated_by
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at ASC;

-- name: SetUserActive :one
UPDATE users
SET is_active = $2, updated_at = now(), updated_by = $3
WHERE id = $1
RETURNING *;
