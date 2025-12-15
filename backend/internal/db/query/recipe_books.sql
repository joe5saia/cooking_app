-- name: ListRecipeBooks :many
SELECT *
FROM recipe_books
ORDER BY name ASC;

-- name: CreateRecipeBook :one
INSERT INTO recipe_books (
  name,
  created_by,
  updated_by
) VALUES (
  $1, $2, $3
)
RETURNING *;

-- name: UpdateRecipeBookByID :one
UPDATE recipe_books
SET name = $2, updated_at = now(), updated_by = $3
WHERE id = $1
RETURNING *;

-- name: DeleteRecipeBookByID :execrows
DELETE FROM recipe_books
WHERE id = $1;

