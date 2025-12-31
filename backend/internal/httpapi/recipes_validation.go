package httpapi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

const maxInt32 = int(^uint32(0) >> 1)

type recipeIngredientRequest struct {
	Position     int      `json:"position"`
	Quantity     *float64 `json:"quantity"`
	QuantityText *string  `json:"quantity_text"`
	Unit         *string  `json:"unit"`
	ItemID       *string  `json:"item_id"`
	ItemName     *string  `json:"item_name"`
	Prep         *string  `json:"prep"`
	Notes        *string  `json:"notes"`
	OriginalText *string  `json:"original_text"`
}

type recipeStepRequest struct {
	StepNumber  int    `json:"step_number"`
	Instruction string `json:"instruction"`
}

type createRecipeRequest struct {
	Title            string                    `json:"title"`
	Servings         int                       `json:"servings"`
	PrepTimeMinutes  int                       `json:"prep_time_minutes"`
	TotalTimeMinutes int                       `json:"total_time_minutes"`
	SourceURL        *string                   `json:"source_url"`
	Notes            *string                   `json:"notes"`
	RecipeBookID     *string                   `json:"recipe_book_id"`
	TagIDs           []string                  `json:"tag_ids"`
	Ingredients      []recipeIngredientRequest `json:"ingredients"`
	Steps            []recipeStepRequest       `json:"steps"`
}

func validateCreateRecipeRequest(req createRecipeRequest) []response.FieldError {
	var errs []response.FieldError

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		errs = append(errs, response.FieldError{Field: "title", Message: "title is required"})
	}
	if req.Servings <= 0 {
		errs = append(errs, response.FieldError{Field: "servings", Message: "servings must be > 0"})
	} else if req.Servings > maxInt32 {
		errs = append(errs, response.FieldError{Field: "servings", Message: "servings is too large"})
	}
	if req.PrepTimeMinutes < 0 {
		errs = append(errs, response.FieldError{Field: "prep_time_minutes", Message: "prep_time_minutes must be >= 0"})
	} else if req.PrepTimeMinutes > maxInt32 {
		errs = append(errs, response.FieldError{Field: "prep_time_minutes", Message: "prep_time_minutes is too large"})
	}
	if req.TotalTimeMinutes < 0 {
		errs = append(errs, response.FieldError{Field: "total_time_minutes", Message: "total_time_minutes must be >= 0"})
	} else if req.TotalTimeMinutes > maxInt32 {
		errs = append(errs, response.FieldError{Field: "total_time_minutes", Message: "total_time_minutes is too large"})
	}

	positionsSeen := map[int]struct{}{}
	for i, ing := range req.Ingredients {
		if ing.Position < 1 {
			errs = append(errs, response.FieldError{
				Field:   fmt.Sprintf("ingredients[%d].position", i),
				Message: "position must be >= 1",
			})
		} else if ing.Position > maxInt32 {
			errs = append(errs, response.FieldError{
				Field:   fmt.Sprintf("ingredients[%d].position", i),
				Message: "position is too large",
			})
		}
		if ing.Position >= 1 {
			if _, ok := positionsSeen[ing.Position]; ok {
				errs = append(errs, response.FieldError{
					Field:   "ingredients",
					Message: "ingredient positions must be unique",
				})
			}
			positionsSeen[ing.Position] = struct{}{}
		}

		itemID := ""
		if ing.ItemID != nil {
			itemID = strings.TrimSpace(*ing.ItemID)
		}
		itemName := ""
		if ing.ItemName != nil {
			itemName = strings.TrimSpace(*ing.ItemName)
		}

		if itemID == "" && itemName == "" {
			errs = append(errs, response.FieldError{
				Field:   fmt.Sprintf("ingredients[%d].item_id", i),
				Message: "item_id or item_name is required",
			})
		}
		if itemID != "" {
			if _, err := uuid.Parse(itemID); err != nil {
				errs = append(errs, response.FieldError{
					Field:   fmt.Sprintf("ingredients[%d].item_id", i),
					Message: "item_id is invalid",
				})
			}
		}
	}

	if len(req.Steps) == 0 {
		errs = append(errs, response.FieldError{Field: "steps", Message: "at least one step is required"})
	} else {
		stepNums := make([]int, 0, len(req.Steps))
		stepSeen := map[int]struct{}{}
		for i, s := range req.Steps {
			if s.StepNumber < 1 {
				errs = append(errs, response.FieldError{
					Field:   fmt.Sprintf("steps[%d].step_number", i),
					Message: "step_number must be >= 1",
				})
			} else if s.StepNumber > maxInt32 {
				errs = append(errs, response.FieldError{
					Field:   fmt.Sprintf("steps[%d].step_number", i),
					Message: "step_number is too large",
				})
			}
			if strings.TrimSpace(s.Instruction) == "" {
				errs = append(errs, response.FieldError{
					Field:   fmt.Sprintf("steps[%d].instruction", i),
					Message: "instruction is required",
				})
			}
			if _, ok := stepSeen[s.StepNumber]; ok {
				errs = append(errs, response.FieldError{Field: "steps", Message: "step numbers must be unique"})
			}
			stepSeen[s.StepNumber] = struct{}{}
			stepNums = append(stepNums, s.StepNumber)
		}
		sort.Ints(stepNums)
		for i := range stepNums {
			want := i + 1
			if stepNums[i] != want {
				errs = append(errs, response.FieldError{Field: "steps", Message: "steps must be numbered consecutively starting at 1"})
				break
			}
		}
	}

	tagSeen := map[string]struct{}{}
	for i, id := range req.TagIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			errs = append(errs, response.FieldError{
				Field:   fmt.Sprintf("tag_ids[%d]", i),
				Message: "tag id is required",
			})
			continue
		}
		if _, ok := tagSeen[id]; ok {
			errs = append(errs, response.FieldError{Field: "tag_ids", Message: "tag_ids must be unique"})
			break
		}
		tagSeen[id] = struct{}{}
	}

	return errs
}
