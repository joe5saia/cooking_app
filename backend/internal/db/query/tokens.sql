-- name: CreateToken :one
INSERT INTO personal_access_tokens (
  user_id,
  name,
  token_hash,
  last_used_at,
  expires_at,
  created_by,
  updated_by
) VALUES (
  $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: ListTokensByUser :many
SELECT *
FROM personal_access_tokens
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: DeleteTokenByIDForUser :execrows
DELETE FROM personal_access_tokens
WHERE id = $1 AND user_id = $2;

-- name: GetTokenUserByHash :one
SELECT
  t.id as token_id,
  t.user_id as token_user_id,
  t.expires_at as token_expires_at,
  u.id as user_id,
  u.username as username,
  u.display_name as display_name,
  u.is_active as is_active
FROM personal_access_tokens t
JOIN users u ON u.id = t.user_id
WHERE t.token_hash = $1;

-- name: TouchTokenLastUsed :exec
UPDATE personal_access_tokens
SET last_used_at = now(), updated_at = now(), updated_by = $2
WHERE id = $1;
