package httpapi

import (
	"testing"

	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

func TestValidateCreateRecipeRequest(t *testing.T) {
	t.Run("valid request has no errors", func(t *testing.T) {
		req := createRecipeRequest{
			Title:            " Chicken Soup ",
			Servings:         4,
			PrepTimeMinutes:  15,
			TotalTimeMinutes: 60,
			TagIDs:           []string{"a"},
			Ingredients: []recipeIngredientRequest{
				{Position: 1, ItemName: stringPtr("chicken")},
			},
			Steps: []recipeStepRequest{
				{StepNumber: 1, Instruction: "boil"},
			},
		}

		errs := validateCreateRecipeRequest(req)
		if len(errs) != 0 {
			t.Fatalf("errs=%v, want none", errs)
		}
	})

	t.Run("requires title", func(t *testing.T) {
		req := createRecipeRequest{
			Title:            " ",
			Servings:         1,
			PrepTimeMinutes:  0,
			TotalTimeMinutes: 0,
			Steps:            []recipeStepRequest{{StepNumber: 1, Instruction: "ok"}},
		}
		errs := validateCreateRecipeRequest(req)
		if !hasFieldError(errs, "title") {
			t.Fatalf("errs=%v, want title error", errs)
		}
	})

	t.Run("requires servings > 0", func(t *testing.T) {
		req := createRecipeRequest{
			Title:            "x",
			Servings:         0,
			PrepTimeMinutes:  0,
			TotalTimeMinutes: 0,
			Steps:            []recipeStepRequest{{StepNumber: 1, Instruction: "ok"}},
		}
		errs := validateCreateRecipeRequest(req)
		if !hasFieldError(errs, "servings") {
			t.Fatalf("errs=%v, want servings error", errs)
		}
	})

	t.Run("requires non-negative times", func(t *testing.T) {
		req := createRecipeRequest{
			Title:            "x",
			Servings:         1,
			PrepTimeMinutes:  -1,
			TotalTimeMinutes: -1,
			Steps:            []recipeStepRequest{{StepNumber: 1, Instruction: "ok"}},
		}
		errs := validateCreateRecipeRequest(req)
		if !hasFieldError(errs, "prep_time_minutes") || !hasFieldError(errs, "total_time_minutes") {
			t.Fatalf("errs=%v, want time errors", errs)
		}
	})

	t.Run("requires at least one step", func(t *testing.T) {
		req := createRecipeRequest{
			Title:            "x",
			Servings:         1,
			PrepTimeMinutes:  0,
			TotalTimeMinutes: 0,
		}
		errs := validateCreateRecipeRequest(req)
		if !hasFieldError(errs, "steps") {
			t.Fatalf("errs=%v, want steps error", errs)
		}
	})

	t.Run("requires consecutive step numbers", func(t *testing.T) {
		req := createRecipeRequest{
			Title:            "x",
			Servings:         1,
			PrepTimeMinutes:  0,
			TotalTimeMinutes: 0,
			Steps: []recipeStepRequest{
				{StepNumber: 2, Instruction: "b"},
				{StepNumber: 1, Instruction: "a"},
				{StepNumber: 4, Instruction: "d"},
			},
		}
		errs := validateCreateRecipeRequest(req)
		if !hasFieldError(errs, "steps") {
			t.Fatalf("errs=%v, want steps error", errs)
		}
	})

	t.Run("requires unique ingredient positions", func(t *testing.T) {
		req := createRecipeRequest{
			Title:            "x",
			Servings:         1,
			PrepTimeMinutes:  0,
			TotalTimeMinutes: 0,
			Ingredients: []recipeIngredientRequest{
				{Position: 1, ItemName: stringPtr("a")},
				{Position: 1, ItemName: stringPtr("b")},
			},
			Steps: []recipeStepRequest{
				{StepNumber: 1, Instruction: "ok"},
			},
		}
		errs := validateCreateRecipeRequest(req)
		if !hasFieldError(errs, "ingredients") {
			t.Fatalf("errs=%v, want ingredients error", errs)
		}
	})

	t.Run("requires item_id or item_name", func(t *testing.T) {
		req := createRecipeRequest{
			Title:            "x",
			Servings:         1,
			PrepTimeMinutes:  0,
			TotalTimeMinutes: 0,
			Ingredients: []recipeIngredientRequest{
				{Position: 1},
			},
			Steps: []recipeStepRequest{
				{StepNumber: 1, Instruction: "ok"},
			},
		}
		errs := validateCreateRecipeRequest(req)
		if !hasFieldError(errs, "ingredients[0].item_id") {
			t.Fatalf("errs=%v, want item error", errs)
		}
	})

	t.Run("rejects invalid item_id", func(t *testing.T) {
		req := createRecipeRequest{
			Title:            "x",
			Servings:         1,
			PrepTimeMinutes:  0,
			TotalTimeMinutes: 0,
			Ingredients: []recipeIngredientRequest{
				{Position: 1, ItemID: stringPtr("not-a-uuid")},
			},
			Steps: []recipeStepRequest{
				{StepNumber: 1, Instruction: "ok"},
			},
		}
		errs := validateCreateRecipeRequest(req)
		if !hasFieldError(errs, "ingredients[0].item_id") {
			t.Fatalf("errs=%v, want item_id error", errs)
		}
	})

	t.Run("requires unique tag ids", func(t *testing.T) {
		req := createRecipeRequest{
			Title:            "x",
			Servings:         1,
			PrepTimeMinutes:  0,
			TotalTimeMinutes: 0,
			TagIDs:           []string{"a", "a"},
			Steps: []recipeStepRequest{
				{StepNumber: 1, Instruction: "ok"},
			},
		}
		errs := validateCreateRecipeRequest(req)
		if !hasFieldError(errs, "tag_ids") {
			t.Fatalf("errs=%v, want tag_ids error", errs)
		}
	})
}

func hasFieldError(errs []response.FieldError, field string) bool {
	for _, err := range errs {
		if err.Field == field {
			return true
		}
	}
	return false
}

// stringPtr returns a string pointer for inline literals.
func stringPtr(value string) *string {
	return &value
}
