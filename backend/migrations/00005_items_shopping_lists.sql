-- +goose Up
CREATE TABLE grocery_aisles (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	name citext NOT NULL,
	sort_group int NOT NULL CONSTRAINT grocery_aisles_sort_group_chk CHECK (sort_group IN (0, 1, 2)),
	sort_order int NOT NULL,
	numeric_value int NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	created_by uuid NOT NULL REFERENCES users (id),
	updated_at timestamptz NOT NULL DEFAULT now(),
	updated_by uuid NOT NULL REFERENCES users (id),
	CONSTRAINT grocery_aisles_name_unique UNIQUE (name)
);

CREATE INDEX grocery_aisles_sort_idx ON grocery_aisles (sort_group, sort_order, name);

CREATE TABLE items (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	name citext NOT NULL,
	store_url text NULL,
	aisle_id uuid NULL REFERENCES grocery_aisles (id) ON DELETE SET NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	created_by uuid NOT NULL REFERENCES users (id),
	updated_at timestamptz NOT NULL DEFAULT now(),
	updated_by uuid NOT NULL REFERENCES users (id),
	CONSTRAINT items_name_unique UNIQUE (name)
);

CREATE INDEX items_name_trgm_idx ON items USING gin (name gin_trgm_ops);
CREATE INDEX items_aisle_id_idx ON items (aisle_id);

CREATE TABLE shopping_lists (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	list_date date NOT NULL,
	name text NOT NULL,
	notes text NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	created_by uuid NOT NULL REFERENCES users (id),
	updated_at timestamptz NOT NULL DEFAULT now(),
	updated_by uuid NOT NULL REFERENCES users (id)
);

CREATE INDEX shopping_lists_date_idx ON shopping_lists (list_date);

CREATE TABLE shopping_list_items (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	shopping_list_id uuid NOT NULL REFERENCES shopping_lists (id) ON DELETE CASCADE,
	item_id uuid NOT NULL REFERENCES items (id),
	unit text NULL,
	quantity numeric NULL,
	quantity_text text NULL,
	is_purchased boolean NOT NULL DEFAULT false,
	purchased_at timestamptz NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	created_by uuid NOT NULL REFERENCES users (id),
	updated_at timestamptz NOT NULL DEFAULT now(),
	updated_by uuid NOT NULL REFERENCES users (id),
	CONSTRAINT shopping_list_items_list_item_unit_unique UNIQUE (shopping_list_id, item_id, unit)
);

CREATE INDEX shopping_list_items_list_id_idx ON shopping_list_items (shopping_list_id);
CREATE INDEX shopping_list_items_item_id_idx ON shopping_list_items (item_id);

ALTER TABLE recipe_ingredients
	ADD COLUMN item_id uuid;

INSERT INTO items (name, created_by, updated_by)
SELECT
	item,
	MIN(created_by) AS created_by,
	MIN(updated_by) AS updated_by
FROM recipe_ingredients
GROUP BY item
ON CONFLICT (name) DO NOTHING;

UPDATE recipe_ingredients ri
SET item_id = i.id
FROM items i
WHERE i.name = ri.item;

ALTER TABLE recipe_ingredients
	ALTER COLUMN item_id SET NOT NULL;

ALTER TABLE recipe_ingredients
	ADD CONSTRAINT recipe_ingredients_item_id_fkey FOREIGN KEY (item_id) REFERENCES items (id);

CREATE INDEX recipe_ingredients_item_id_idx ON recipe_ingredients (item_id);

ALTER TABLE recipe_ingredients
	DROP COLUMN item;

-- +goose Down
ALTER TABLE recipe_ingredients
	ADD COLUMN item text;

UPDATE recipe_ingredients ri
SET item = i.name
FROM items i
WHERE i.id = ri.item_id;

ALTER TABLE recipe_ingredients
	ALTER COLUMN item SET NOT NULL;

ALTER TABLE recipe_ingredients
	DROP CONSTRAINT recipe_ingredients_item_id_fkey;

DROP INDEX recipe_ingredients_item_id_idx;

ALTER TABLE recipe_ingredients
	DROP COLUMN item_id;

DROP TABLE shopping_list_items;
DROP TABLE shopping_lists;
DROP TABLE items;
DROP TABLE grocery_aisles;
