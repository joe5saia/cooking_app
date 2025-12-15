-- name: CreateSession :one
INSERT INTO sessions (
  user_id,
  token_hash,
  expires_at,
  last_seen_at,
  created_by,
  updated_by
) VALUES (
  $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions
WHERE token_hash = $1;

-- name: GetSessionUserByTokenHash :one
SELECT
  s.id as session_id,
  s.user_id as session_user_id,
  s.expires_at as session_expires_at,
  s.last_seen_at as session_last_seen_at,
  u.id as user_id,
  u.username as username,
  u.display_name as display_name,
  u.is_active as is_active
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1;

