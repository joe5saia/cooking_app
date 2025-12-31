-- name: ListItems :many
SELECT
  i.id,
  i.name,
  i.store_url,
  i.aisle_id,
  a.name AS aisle_name,
  a.sort_group AS aisle_sort_group,
  a.sort_order AS aisle_sort_order,
  a.numeric_value AS aisle_numeric_value,
  i.created_at,
  i.created_by,
  i.updated_at,
  i.updated_by
FROM items i
LEFT JOIN grocery_aisles a ON a.id = i.aisle_id
WHERE (sqlc.arg(q)::text = '' OR i.name ILIKE ('%' || sqlc.arg(q)::text || '%'))
ORDER BY i.name ASC
LIMIT sqlc.arg(page_limit);

-- name: GetItemByID :one
SELECT
  i.id,
  i.name,
  i.store_url,
  i.aisle_id,
  a.name AS aisle_name,
  a.sort_group AS aisle_sort_group,
  a.sort_order AS aisle_sort_order,
  a.numeric_value AS aisle_numeric_value,
  i.created_at,
  i.created_by,
  i.updated_at,
  i.updated_by
FROM items i
LEFT JOIN grocery_aisles a ON a.id = i.aisle_id
WHERE i.id = $1;

-- name: GetItemByName :one
SELECT
  id,
  name,
  store_url,
  aisle_id,
  created_at,
  created_by,
  updated_at,
  updated_by
FROM items
WHERE name = $1;

-- name: CreateItem :one
INSERT INTO items (
  name,
  store_url,
  aisle_id,
  created_by,
  updated_by
) VALUES (
  $1, $2, $3, $4, $5
)
RETURNING *;

-- name: UpdateItemByID :one
UPDATE items
SET name = $2,
    store_url = $3,
    aisle_id = $4,
    updated_at = now(),
    updated_by = $5
WHERE id = $1
RETURNING *;

-- name: DeleteItemByID :execrows
DELETE FROM items
WHERE id = $1;
