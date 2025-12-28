-- +goose Up
CREATE TABLE meal_plan_entries (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	plan_date date NOT NULL,
	recipe_id uuid NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
	created_at timestamptz NOT NULL DEFAULT now(),
	created_by uuid NOT NULL REFERENCES users (id),
	updated_at timestamptz NOT NULL DEFAULT now(),
	updated_by uuid NOT NULL REFERENCES users (id),
	CONSTRAINT meal_plan_entries_user_date_recipe_unique UNIQUE (user_id, plan_date, recipe_id)
);

CREATE INDEX meal_plan_entries_user_date_idx ON meal_plan_entries (user_id, plan_date);

-- +goose Down
DROP TABLE meal_plan_entries;
