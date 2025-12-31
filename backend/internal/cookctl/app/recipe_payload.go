package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
	"golang.org/x/term"
)

// recipeUpsertPayload is the API payload for recipe create and update requests.
type recipeUpsertPayload struct {
	Title            string                   `json:"title"`
	Servings         int                      `json:"servings"`
	PrepTimeMinutes  int                      `json:"prep_time_minutes"`
	TotalTimeMinutes int                      `json:"total_time_minutes"`
	SourceURL        *string                  `json:"source_url"`
	Notes            *string                  `json:"notes"`
	RecipeBookID     *string                  `json:"recipe_book_id"`
	TagIDs           []string                 `json:"tag_ids"`
	Ingredients      []recipeIngredientUpsert `json:"ingredients"`
	Steps            []recipeStepUpsert       `json:"steps"`
}

// recipeIngredientUpsert captures an ingredient payload for recipe upserts.
type recipeIngredientUpsert struct {
	Position     int      `json:"position"`
	Quantity     *float64 `json:"quantity"`
	QuantityText *string  `json:"quantity_text"`
	Unit         *string  `json:"unit"`
	ItemID       *string  `json:"item_id"`
	ItemName     string   `json:"item_name"`
	Prep         *string  `json:"prep"`
	Notes        *string  `json:"notes"`
	OriginalText *string  `json:"original_text"`
}

// recipeStepUpsert captures a recipe step payload for recipe upserts.
type recipeStepUpsert struct {
	StepNumber  int    `json:"step_number"`
	Instruction string `json:"instruction"`
}

// readJSONFile reads a JSON object from a file.
func readJSONFile(path string) (json.RawMessage, error) {
	//nolint:gosec // Path is user-supplied by design for reading recipe payloads.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("file is empty")
	}
	return readJSONBytes(raw)
}

// readJSONReader reads a JSON object from an io.Reader.
func readJSONReader(r io.Reader) (json.RawMessage, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("input is empty")
	}
	return readJSONBytes(raw)
}

// readJSONBytes validates and returns a JSON object payload.
func readJSONBytes(raw []byte) (json.RawMessage, error) {
	var payload map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	if len(payload) == 0 {
		return nil, errors.New("json object is empty")
	}
	return json.RawMessage(raw), nil
}

// readRawJSONFile reads JSON bytes without enforcing object-only payloads.
func readRawJSONFile(path string) ([]byte, error) {
	//nolint:gosec // Path is user-supplied by design for reading recipe payloads.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return readRawJSONBytes(raw)
}

// readRawJSONReader reads JSON bytes from a reader without enforcing object-only payloads.
func readRawJSONReader(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	return readRawJSONBytes(raw)
}

// readRawJSONBytes validates that JSON is non-empty.
func readRawJSONBytes(raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("input is empty")
	}
	return raw, nil
}

// splitJSONPayloads splits JSON into one or more object payloads.
func splitJSONPayloads(raw []byte) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("input is empty")
	}
	if trimmed[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, fmt.Errorf("invalid json array: %w", err)
		}
		if len(items) == 0 {
			return nil, errors.New("json array is empty")
		}
		return items, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		return nil, fmt.Errorf("invalid json object: %w", err)
	}
	if len(payload) == 0 {
		return nil, errors.New("json object is empty")
	}
	return []json.RawMessage{json.RawMessage(trimmed)}, nil
}

// recipeTitleFromJSON extracts the title field for duplicate checks.
func recipeTitleFromJSON(raw json.RawMessage) (string, error) {
	var payload struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("invalid recipe payload: %w", err)
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		return "", errors.New("title is required")
	}
	return title, nil
}

// isTerminal reports whether the reader is a terminal.
func isTerminal(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

// recipeTemplatePayload returns a starter payload with placeholders.
func recipeTemplatePayload() recipeUpsertPayload {
	ingredient := "Ingredient"
	step := "Add steps here"
	return recipeUpsertPayload{
		Title:            "New Recipe",
		Servings:         1,
		PrepTimeMinutes:  0,
		TotalTimeMinutes: 0,
		RecipeBookID:     nil,
		TagIDs:           []string{},
		Ingredients: []recipeIngredientUpsert{
			{
				Position:     1,
				ItemName:     ingredient,
				OriginalText: &ingredient,
			},
		},
		Steps: []recipeStepUpsert{
			{
				StepNumber:  1,
				Instruction: step,
			},
		},
	}
}

// buildRecipePayloadInteractive prompts for recipe fields and returns a payload.
func buildRecipePayloadInteractive(stdin io.Reader, stderr io.Writer, ctx context.Context, api *client.Client) (recipeUpsertPayload, error) {
	prompter := newPromptInput(stdin, stderr)
	title, err := prompter.askRequired("Title")
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	servings, err := prompter.askInt("Servings", 1, 1)
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	prepMinutes, err := prompter.askInt("Prep time minutes", 0, 0)
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	totalMinutes, err := prompter.askInt("Total time minutes", 0, 0)
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	sourceURL, err := prompter.askOptional("Source URL (optional)")
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	notes, err := prompter.askOptional("Notes (optional)")
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	bookRaw, err := prompter.askOptional("Recipe book (name or id, optional)")
	if err != nil {
		return recipeUpsertPayload{}, err
	}

	var bookID *string
	if bookRaw != nil && strings.TrimSpace(*bookRaw) != "" {
		resolvedID, resolveErr := resolveBookID(ctx, api, *bookRaw)
		if resolveErr != nil {
			return recipeUpsertPayload{}, resolveErr
		}
		bookID = &resolvedID
	}

	tagRaw, err := prompter.askOptional("Tags (comma-separated, optional)")
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	tagIDs := []string{}
	if tagRaw != nil && strings.TrimSpace(*tagRaw) != "" {
		tagNames := splitCommaSeparated(*tagRaw)
		resolvedIDs, resolveErr := resolveTagIDsByName(ctx, api, tagNames, true)
		if resolveErr != nil {
			return recipeUpsertPayload{}, resolveErr
		}
		tagIDs = resolvedIDs
	}

	writeLine(stderr, "Enter ingredients (blank to finish):")
	ingredients, err := prompter.askIngredients()
	if err != nil {
		return recipeUpsertPayload{}, err
	}

	writeLine(stderr, "Enter steps (blank to finish):")
	steps, err := prompter.askSteps()
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	if len(steps) == 0 {
		return recipeUpsertPayload{}, errors.New("at least one step is required")
	}

	return recipeUpsertPayload{
		Title:            title,
		Servings:         servings,
		PrepTimeMinutes:  prepMinutes,
		TotalTimeMinutes: totalMinutes,
		SourceURL:        sourceURL,
		Notes:            notes,
		RecipeBookID:     bookID,
		TagIDs:           tagIDs,
		Ingredients:      ingredients,
		Steps:            steps,
	}, nil
}

// splitCommaSeparated splits comma-separated values into trimmed fields.
func splitCommaSeparated(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// promptInput wraps interactive prompting helpers.
type promptInput struct {
	reader *bufio.Reader
	writer io.Writer
}

// newPromptInput creates a prompt helper for interactive input.
func newPromptInput(r io.Reader, w io.Writer) *promptInput {
	return &promptInput{
		reader: bufio.NewReader(r),
		writer: w,
	}
}

// ask prompts for a single line of input.
func (p *promptInput) ask(label string) (string, error) {
	if _, err := fmt.Fprintf(p.writer, "%s: ", label); err != nil {
		return "", err
	}
	line, err := p.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if errors.Is(err, io.EOF) && line == "" {
		return "", io.EOF
	}
	return line, nil
}

// askRequired prompts until a non-empty value is provided.
func (p *promptInput) askRequired(label string) (string, error) {
	for {
		value, err := p.ask(label)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(value) != "" {
			return value, nil
		}
	}
}

// askOptional prompts for an optional value.
func (p *promptInput) askOptional(label string) (*string, error) {
	value, err := p.ask(label)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	return &trimmed, nil
}

// askInt prompts for an integer with defaults and a minimum.
func (p *promptInput) askInt(label string, defaultValue, minValue int) (int, error) {
	for {
		value, err := p.ask(fmt.Sprintf("%s [%d]", label, defaultValue))
		if err != nil {
			return 0, err
		}
		if strings.TrimSpace(value) == "" {
			return defaultValue, nil
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			writeLine(p.writer, "Enter a valid number.")
			continue
		}
		if parsed < minValue {
			writeLine(p.writer, fmt.Sprintf("Value must be >= %d.", minValue))
			continue
		}
		return parsed, nil
	}
}

// askIngredients collects ingredient lines.
func (p *promptInput) askIngredients() ([]recipeIngredientUpsert, error) {
	ingredients := []recipeIngredientUpsert{}
	for {
		line, err := p.ask("Ingredient")
		if err != nil {
			if errors.Is(err, io.EOF) {
				return ingredients, nil
			}
			return nil, err
		}
		if strings.TrimSpace(line) == "" {
			return ingredients, nil
		}
		trimmed := strings.TrimSpace(line)
		ingredients = append(ingredients, recipeIngredientUpsert{
			Position:     len(ingredients) + 1,
			ItemName:     trimmed,
			OriginalText: &trimmed,
		})
	}
}

// askSteps collects recipe steps.
func (p *promptInput) askSteps() ([]recipeStepUpsert, error) {
	steps := []recipeStepUpsert{}
	for {
		line, err := p.ask("Step")
		if err != nil {
			if errors.Is(err, io.EOF) {
				return steps, nil
			}
			return nil, err
		}
		if strings.TrimSpace(line) == "" {
			return steps, nil
		}
		trimmed := strings.TrimSpace(line)
		steps = append(steps, recipeStepUpsert{
			StepNumber:  len(steps) + 1,
			Instruction: trimmed,
		})
	}
}

// toUpsertPayload converts a recipe detail response into an upsert payload.
func toUpsertPayload(recipe client.RecipeDetail) recipeUpsertPayload {
	tagIDs := make([]string, 0, len(recipe.Tags))
	for _, tag := range recipe.Tags {
		tagIDs = append(tagIDs, tag.ID)
	}

	ingredients := make([]recipeIngredientUpsert, 0, len(recipe.Ingredients))
	for _, ingredient := range recipe.Ingredients {
		itemID := stringPtrIfNotEmpty(ingredient.Item.ID)
		ingredients = append(ingredients, recipeIngredientUpsert{
			Position:     ingredient.Position,
			Quantity:     ingredient.Quantity,
			QuantityText: ingredient.QuantityText,
			Unit:         ingredient.Unit,
			ItemID:       itemID,
			ItemName:     ingredient.Item.Name,
			Prep:         ingredient.Prep,
			Notes:        ingredient.Notes,
			OriginalText: ingredient.OriginalText,
		})
	}

	steps := make([]recipeStepUpsert, 0, len(recipe.Steps))
	for _, step := range recipe.Steps {
		steps = append(steps, recipeStepUpsert{
			StepNumber:  step.StepNumber,
			Instruction: step.Instruction,
		})
	}

	return recipeUpsertPayload{
		Title:            recipe.Title,
		Servings:         recipe.Servings,
		PrepTimeMinutes:  recipe.PrepTimeMinutes,
		TotalTimeMinutes: recipe.TotalTimeMinutes,
		SourceURL:        recipe.SourceURL,
		Notes:            recipe.Notes,
		RecipeBookID:     recipe.RecipeBookID,
		TagIDs:           tagIDs,
		Ingredients:      ingredients,
		Steps:            steps,
	}
}
