-- name: ListShoppingListItemsByListID :many
SELECT
  sli.id,
  sli.shopping_list_id,
  sli.item_id,
  sli.unit,
  sli.quantity,
  sli.quantity_text,
  sli.is_purchased,
  sli.purchased_at,
  i.name AS item_name,
  i.store_url AS item_store_url,
  i.aisle_id AS item_aisle_id,
  a.name AS aisle_name,
  a.sort_group AS aisle_sort_group,
  a.sort_order AS aisle_sort_order,
  a.numeric_value AS aisle_numeric_value
FROM shopping_list_items sli
JOIN shopping_lists sl ON sl.id = sli.shopping_list_id
JOIN items i ON i.id = sli.item_id
LEFT JOIN grocery_aisles a ON a.id = i.aisle_id
WHERE sli.shopping_list_id = sqlc.arg(shopping_list_id)
  AND sl.created_by = sqlc.arg(user_id)
ORDER BY
  COALESCE(a.sort_group, 2) ASC,
  COALESCE(a.sort_order, 0) ASC,
  COALESCE(a.numeric_value, 0) ASC,
  a.name ASC,
  i.name ASC;

-- name: UpsertShoppingListItem :one
WITH list_scope AS (
  SELECT sl.id
  FROM shopping_lists sl
  WHERE sl.id = sqlc.arg(shopping_list_id)
    AND sl.created_by = sqlc.arg(user_id)
)
INSERT INTO shopping_list_items (
  shopping_list_id,
  item_id,
  unit,
  quantity,
  quantity_text,
  is_purchased,
  purchased_at,
  created_by,
  updated_by
)
SELECT
  list_scope.id,
  sqlc.arg(item_id),
  sqlc.arg(unit),
  sqlc.arg(quantity),
  sqlc.arg(quantity_text),
  false,
  NULL,
  sqlc.arg(created_by),
  sqlc.arg(updated_by)
FROM list_scope
ON CONFLICT (shopping_list_id, item_id, unit) DO UPDATE
SET quantity = CASE
    WHEN shopping_list_items.quantity IS NULL AND EXCLUDED.quantity IS NULL THEN NULL
    WHEN shopping_list_items.quantity IS NULL THEN EXCLUDED.quantity
    WHEN EXCLUDED.quantity IS NULL THEN shopping_list_items.quantity
    ELSE shopping_list_items.quantity + EXCLUDED.quantity
  END,
  quantity_text = COALESCE(EXCLUDED.quantity_text, shopping_list_items.quantity_text),
  is_purchased = false,
  purchased_at = NULL,
  updated_at = now(),
  updated_by = EXCLUDED.updated_by
RETURNING id, shopping_list_id, item_id, unit, quantity, quantity_text, is_purchased, purchased_at;

-- name: UpdateShoppingListItemPurchased :one
UPDATE shopping_list_items AS sli
SET is_purchased = sqlc.arg(is_purchased),
    purchased_at = CASE WHEN sqlc.arg(is_purchased) THEN now() ELSE NULL END,
    updated_at = now(),
    updated_by = sqlc.arg(updated_by)
FROM shopping_lists sl
JOIN items i ON i.id = sli.item_id
LEFT JOIN grocery_aisles a ON a.id = i.aisle_id
WHERE sli.id = sqlc.arg(id)
  AND sli.shopping_list_id = sqlc.arg(shopping_list_id)
  AND sli.shopping_list_id = sl.id
  AND sl.created_by = sqlc.arg(user_id)
RETURNING
  sli.id,
  sli.shopping_list_id,
  sli.item_id,
  sli.unit,
  sli.quantity,
  sli.quantity_text,
  sli.is_purchased,
  sli.purchased_at,
  i.name AS item_name,
  i.store_url AS item_store_url,
  i.aisle_id AS item_aisle_id,
  a.name AS aisle_name,
  a.sort_group AS aisle_sort_group,
  a.sort_order AS aisle_sort_order,
  a.numeric_value AS aisle_numeric_value;

-- name: DeleteShoppingListItemByID :execrows
DELETE FROM shopping_list_items sli
USING shopping_lists sl
WHERE sli.id = sqlc.arg(id)
  AND sli.shopping_list_id = sqlc.arg(shopping_list_id)
  AND sli.shopping_list_id = sl.id
  AND sl.created_by = sqlc.arg(user_id);

-- name: ListRecipeIngredientsByRecipeIDs :many
SELECT
  ri.item_id,
  ri.quantity,
  ri.quantity_text,
  ri.unit
FROM recipe_ingredients ri
JOIN recipes r ON r.id = ri.recipe_id
WHERE ri.recipe_id = ANY(sqlc.arg(recipe_ids)::uuid[])
  AND r.deleted_at IS NULL
ORDER BY ri.recipe_id ASC, ri.position ASC;

-- name: ListRecipeIngredientsByMealPlanDate :many
SELECT
  ri.item_id,
  ri.quantity,
  ri.quantity_text,
  ri.unit
FROM meal_plan_entries mpe
JOIN recipes r ON r.id = mpe.recipe_id
JOIN recipe_ingredients ri ON ri.recipe_id = r.id
WHERE mpe.user_id = sqlc.arg(user_id)
  AND mpe.plan_date = sqlc.arg(plan_date)
  AND r.deleted_at IS NULL
ORDER BY mpe.plan_date ASC, ri.position ASC;
