-- name: ListGroceryAisles :many
SELECT
  id,
  name,
  sort_group,
  sort_order,
  numeric_value,
  created_at,
  created_by,
  updated_at,
  updated_by
FROM grocery_aisles
ORDER BY sort_group ASC, sort_order ASC, name ASC;

-- name: GetGroceryAisleByID :one
SELECT
  id,
  name,
  sort_group,
  sort_order,
  numeric_value,
  created_at,
  created_by,
  updated_at,
  updated_by
FROM grocery_aisles
WHERE id = $1;

-- name: CreateGroceryAisle :one
INSERT INTO grocery_aisles (
  name,
  sort_group,
  sort_order,
  numeric_value,
  created_by,
  updated_by
) VALUES (
  $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: UpdateGroceryAisleByID :one
UPDATE grocery_aisles
SET name = $2,
    sort_group = $3,
    sort_order = $4,
    numeric_value = $5,
    updated_at = now(),
    updated_by = $6
WHERE id = $1
RETURNING *;

-- name: DeleteGroceryAisleByID :execrows
DELETE FROM grocery_aisles
WHERE id = $1;
