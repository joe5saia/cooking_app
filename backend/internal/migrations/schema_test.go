package migrations_test

import (
	"context"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

func TestSchema_MigrationsCreateExpectedArtifacts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)

	postgres.MigrateUp(ctx, t, db)

	assertRegclassExists(ctx, t, db, "public.users")
	assertRegclassExists(ctx, t, db, "public.recipe_books")
	assertRegclassExists(ctx, t, db, "public.tags")
	assertRegclassExists(ctx, t, db, "public.recipes")
	assertRegclassExists(ctx, t, db, "public.recipe_ingredients")
	assertRegclassExists(ctx, t, db, "public.recipe_steps")
	assertRegclassExists(ctx, t, db, "public.recipe_tags")
	assertRegclassExists(ctx, t, db, "public.personal_access_tokens")
	assertRegclassExists(ctx, t, db, "public.sessions")

	assertColumnUDT(ctx, t, db, "users", "username", "citext")
	assertColumnUDT(ctx, t, db, "recipe_books", "name", "citext")
	assertColumnUDT(ctx, t, db, "tags", "name", "citext")

	assertColumnExists(ctx, t, db, "recipes", "deleted_at")

	assertRegclassExists(ctx, t, db, "public.recipes_updated_at_idx")
	assertRegclassExists(ctx, t, db, "public.recipes_deleted_at_idx")
	assertRegclassExists(ctx, t, db, "public.recipes_title_trgm_idx")

	assertConstraintExists(ctx, t, db, "recipe_steps_recipe_id_step_number_unique")
	assertConstraintExists(ctx, t, db, "personal_access_tokens_token_hash_unique")
	assertConstraintExists(ctx, t, db, "sessions_token_hash_unique")

	assertFKHasOnDeleteCascade(ctx, t, db, "recipe_ingredients_recipe_id_fkey")
	assertFKHasOnDeleteCascade(ctx, t, db, "recipe_steps_recipe_id_fkey")
	assertFKHasOnDeleteCascade(ctx, t, db, "recipe_tags_recipe_id_fkey")
	assertFKHasOnDeleteCascade(ctx, t, db, "recipe_tags_tag_id_fkey")
	assertFKHasOnDeleteCascade(ctx, t, db, "personal_access_tokens_user_id_fkey")
	assertFKHasOnDeleteCascade(ctx, t, db, "sessions_user_id_fkey")
}
