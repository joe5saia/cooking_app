-- name: CreateRecipe :one
INSERT INTO recipes (
  title,
  servings,
  prep_time_minutes,
  total_time_minutes,
  source_url,
  notes,
  recipe_book_id,
  deleted_at,
  created_by,
  updated_by
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, NULL, $8, $9
)
RETURNING *;

-- name: GetRecipeByID :one
SELECT *
FROM recipes
WHERE id = $1;

-- name: ListRecipeIngredientsByRecipeID :many
SELECT *
FROM recipe_ingredients
WHERE recipe_id = $1
ORDER BY position ASC;

-- name: ListRecipeStepsByRecipeID :many
SELECT *
FROM recipe_steps
WHERE recipe_id = $1
ORDER BY step_number ASC;

-- name: ListRecipeTagsByRecipeID :many
SELECT
  t.id,
  t.name
FROM recipe_tags rt
JOIN tags t ON t.id = rt.tag_id
WHERE rt.recipe_id = $1
ORDER BY t.name ASC;

-- name: CreateRecipeIngredient :exec
INSERT INTO recipe_ingredients (
  recipe_id,
  position,
  quantity,
  quantity_text,
  unit,
  item,
  prep,
  notes,
  original_text,
  created_by,
  updated_by
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
);

-- name: CreateRecipeStep :exec
INSERT INTO recipe_steps (
  recipe_id,
  step_number,
  instruction,
  created_by,
  updated_by
) VALUES (
  $1, $2, $3, $4, $5
);

-- name: CreateRecipeTag :exec
INSERT INTO recipe_tags (
  recipe_id,
  tag_id,
  created_by,
  updated_by
) VALUES (
  $1, $2, $3, $4
);

-- name: CountTagsByIDs :one
SELECT COUNT(*)::int AS count
FROM tags
WHERE id = ANY($1::uuid[]);

-- name: ListRecipes :many
SELECT
  r.id,
  r.title,
  r.servings,
  r.prep_time_minutes,
  r.total_time_minutes,
  r.source_url,
  r.notes,
  r.recipe_book_id,
  r.deleted_at,
  r.updated_at
FROM recipes r
WHERE
  (sqlc.arg(q)::text = '' OR r.title ILIKE ('%' || sqlc.arg(q)::text || '%'))
  AND (sqlc.arg(book_id)::uuid IS NULL OR r.recipe_book_id = sqlc.arg(book_id)::uuid)
  AND (sqlc.arg(tag_id)::uuid IS NULL OR EXISTS (
    SELECT 1
    FROM recipe_tags rt
    WHERE rt.recipe_id = r.id AND rt.tag_id = sqlc.arg(tag_id)::uuid
  ))
  AND (sqlc.arg(include_deleted)::boolean OR r.deleted_at IS NULL)
  AND (
    sqlc.arg(cursor_updated_at)::timestamptz IS NULL
    OR (r.updated_at, r.id) < (sqlc.arg(cursor_updated_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
  )
ORDER BY r.updated_at DESC, r.id DESC
LIMIT sqlc.arg(page_limit);

-- name: ListRecipeTagsByRecipeIDs :many
SELECT
  rt.recipe_id,
  t.id,
  t.name
FROM recipe_tags rt
JOIN tags t ON t.id = rt.tag_id
WHERE rt.recipe_id = ANY($1::uuid[])
ORDER BY rt.recipe_id, t.name ASC;

-- name: SoftDeleteRecipeByID :execrows
UPDATE recipes
SET deleted_at = now(),
    updated_at = now(),
    updated_by = $2
WHERE id = $1;

-- name: RestoreRecipeByID :execrows
UPDATE recipes
SET deleted_at = NULL,
    updated_at = now(),
    updated_by = $2
WHERE id = $1;

-- name: GetRecipeDeletedAtByID :one
SELECT deleted_at
FROM recipes
WHERE id = $1;

-- name: UpdateRecipeByID :one
UPDATE recipes
SET title = $2,
    servings = $3,
    prep_time_minutes = $4,
    total_time_minutes = $5,
    source_url = $6,
    notes = $7,
    recipe_book_id = $8,
    updated_at = now(),
    updated_by = $9
WHERE id = $1
RETURNING *;

-- name: DeleteRecipeIngredientsByRecipeID :exec
DELETE FROM recipe_ingredients
WHERE recipe_id = $1;

-- name: DeleteRecipeStepsByRecipeID :exec
DELETE FROM recipe_steps
WHERE recipe_id = $1;

-- name: DeleteRecipeTagsByRecipeID :exec
DELETE FROM recipe_tags
WHERE recipe_id = $1;
