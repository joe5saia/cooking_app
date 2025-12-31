-- name: ListShoppingListsByDateRange :many
SELECT
  id,
  list_date,
  name,
  notes,
  created_at,
  updated_at
FROM shopping_lists
WHERE created_by = sqlc.arg(user_id)
  AND list_date >= sqlc.arg(start_date)
  AND list_date <= sqlc.arg(end_date)
ORDER BY list_date ASC, created_at ASC;

-- name: CreateShoppingList :one
INSERT INTO shopping_lists (
  list_date,
  name,
  notes,
  created_by,
  updated_by
) VALUES (
  sqlc.arg(list_date),
  sqlc.arg(name),
  sqlc.arg(notes),
  sqlc.arg(created_by),
  sqlc.arg(updated_by)
)
RETURNING id, list_date, name, notes, created_at, updated_at;

-- name: GetShoppingListByID :one
SELECT
  id,
  list_date,
  name,
  notes,
  created_at,
  updated_at
FROM shopping_lists
WHERE id = sqlc.arg(id)
  AND created_by = sqlc.arg(user_id);

-- name: UpdateShoppingListByID :one
UPDATE shopping_lists
SET list_date = sqlc.arg(list_date),
    name = sqlc.arg(name),
    notes = sqlc.arg(notes),
    updated_at = now(),
    updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id)
  AND created_by = sqlc.arg(user_id)
RETURNING id, list_date, name, notes, created_at, updated_at;

-- name: DeleteShoppingListByID :execrows
DELETE FROM shopping_lists
WHERE id = sqlc.arg(id)
  AND created_by = sqlc.arg(user_id);
