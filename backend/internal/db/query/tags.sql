-- name: ListTags :many
SELECT *
FROM tags
ORDER BY name ASC;

-- name: CreateTag :one
INSERT INTO tags (
  name,
  created_by,
  updated_by
) VALUES (
  $1, $2, $3
)
RETURNING *;

-- name: UpdateTagByID :one
UPDATE tags
SET name = $2, updated_at = now(), updated_by = $3
WHERE id = $1
RETURNING *;

-- name: DeleteTagByID :execrows
DELETE FROM tags
WHERE id = $1;

