package app

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
)

func TestReadJSONBytes(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"title":"Pasta"}`)
	parsed, err := readJSONBytes(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(parsed) != string(payload) {
		t.Fatalf("payload = %s, want %s", parsed, payload)
	}

	_, err = readJSONBytes([]byte("{}"))
	if err == nil {
		t.Fatal("expected error for empty json object")
	}
}

func TestReadRawJSONBytes(t *testing.T) {
	t.Parallel()

	_, err := readRawJSONBytes([]byte("   "))
	if err == nil {
		t.Fatal("expected error for empty json input")
	}

	raw, err := readRawJSONBytes([]byte("[]"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != "[]" {
		t.Fatalf("raw = %s, want []", raw)
	}
}

func TestSplitJSONPayloads(t *testing.T) {
	t.Parallel()

	payloads, err := splitJSONPayloads([]byte(`[{"title":"A"},{"title":"B"}]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(payloads) != 2 {
		t.Fatalf("payload count = %d, want 2", len(payloads))
	}

	single, err := splitJSONPayloads([]byte(`{"title":"A"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(single) != 1 {
		t.Fatalf("payload count = %d, want 1", len(single))
	}
}

func TestRecipeTitleFromJSON(t *testing.T) {
	t.Parallel()

	const recipeTitle = "Soup"

	title, err := recipeTitleFromJSON(json.RawMessage(`{"title":"` + recipeTitle + `"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if title != recipeTitle {
		t.Fatalf("title = %s, want %s", title, recipeTitle)
	}

	_, err = recipeTitleFromJSON(json.RawMessage(`{"title":""}`))
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestRecipeTemplatePayload(t *testing.T) {
	t.Parallel()

	payload := recipeTemplatePayload()
	if payload.Title == "" {
		t.Fatal("expected template title")
	}
	if len(payload.Ingredients) != 1 {
		t.Fatalf("ingredient count = %d, want 1", len(payload.Ingredients))
	}
	if len(payload.Steps) != 1 {
		t.Fatalf("step count = %d, want 1", len(payload.Steps))
	}
}

func TestBuildRecipePayloadInteractive(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"Test Recipe",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"Salt",
		"",
		"Mix",
		"",
	}, "\n")

	payload, err := buildRecipePayloadInteractive(bytes.NewBufferString(input), &bytes.Buffer{}, context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.Title != "Test Recipe" {
		t.Fatalf("title = %s, want Test Recipe", payload.Title)
	}
	if len(payload.Ingredients) != 1 {
		t.Fatalf("ingredient count = %d, want 1", len(payload.Ingredients))
	}
	if len(payload.Steps) != 1 {
		t.Fatalf("step count = %d, want 1", len(payload.Steps))
	}
}

func TestToUpsertPayload(t *testing.T) {
	t.Parallel()

	const tagID = "tag-1"

	quantity := 2.0
	unit := "cups"
	itemID := "item-1"
	ingredientText := "2 cups flour"
	recipe := client.RecipeDetail{
		Title:            "Bread",
		Servings:         2,
		PrepTimeMinutes:  10,
		TotalTimeMinutes: 60,
		Tags: []client.RecipeTag{
			{ID: tagID, Name: "Baking"},
		},
		Ingredients: []client.RecipeIngredient{
			{
				Position:     1,
				Quantity:     &quantity,
				Unit:         &unit,
				Item:         client.Item{ID: itemID, Name: "Flour"},
				OriginalText: &ingredientText,
			},
		},
		Steps: []client.RecipeStep{
			{StepNumber: 1, Instruction: "Mix"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	payload := toUpsertPayload(recipe)
	if len(payload.TagIDs) != 1 || payload.TagIDs[0] != tagID {
		t.Fatalf("tag IDs = %v, want [%s]", payload.TagIDs, tagID)
	}
	if payload.Ingredients[0].ItemID == nil || *payload.Ingredients[0].ItemID != itemID {
		t.Fatalf("item id = %v, want %s", payload.Ingredients[0].ItemID, itemID)
	}
	if payload.Ingredients[0].Quantity == nil || *payload.Ingredients[0].Quantity != quantity {
		t.Fatalf("quantity = %v, want %v", payload.Ingredients[0].Quantity, quantity)
	}
	if payload.Ingredients[0].Unit == nil || *payload.Ingredients[0].Unit != unit {
		t.Fatalf("unit = %v, want %s", payload.Ingredients[0].Unit, unit)
	}
}
