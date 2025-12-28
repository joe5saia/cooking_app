-- name: ListMealPlanEntriesByRange :many
SELECT
  mpe.id,
  mpe.plan_date,
  mpe.recipe_id,
  r.title
FROM meal_plan_entries mpe
JOIN recipes r ON r.id = mpe.recipe_id
WHERE mpe.user_id = sqlc.arg(user_id)
  AND mpe.plan_date >= sqlc.arg(start_date)
  AND mpe.plan_date <= sqlc.arg(end_date)
  AND r.deleted_at IS NULL
ORDER BY mpe.plan_date ASC, mpe.created_at ASC;

-- name: CreateMealPlanEntry :one
WITH candidate AS (
  SELECT
    r.id AS recipe_id,
    r.title
  FROM recipes r
  WHERE r.id = sqlc.arg(recipe_id)
    AND r.deleted_at IS NULL
),
inserted AS (
  INSERT INTO meal_plan_entries (
    user_id,
    plan_date,
    recipe_id,
    created_by,
    updated_by
  )
  SELECT
    sqlc.arg(user_id),
    sqlc.arg(plan_date),
    c.recipe_id,
    sqlc.arg(created_by),
    sqlc.arg(updated_by)
  FROM candidate c
  RETURNING id, plan_date, recipe_id
)
SELECT
  inserted.id,
  inserted.plan_date,
  inserted.recipe_id,
  c.title
FROM inserted
JOIN candidate c ON c.recipe_id = inserted.recipe_id;

-- name: DeleteMealPlanEntry :execrows
DELETE FROM meal_plan_entries
WHERE user_id = sqlc.arg(user_id)
  AND plan_date = sqlc.arg(plan_date)
  AND recipe_id = sqlc.arg(recipe_id);
