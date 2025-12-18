package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

type fakeRecipeWorkflows struct {
	countTagsByIDs func(ctx context.Context, ids []pgtype.UUID) (int32, error)
	withinTx       func(ctx context.Context, fn func(q recipeWorkflowQueries) error) error
}

func (f fakeRecipeWorkflows) CountTagsByIDs(ctx context.Context, ids []pgtype.UUID) (int32, error) {
	if f.countTagsByIDs == nil {
		return 0, errors.New("CountTagsByIDs not implemented")
	}
	return f.countTagsByIDs(ctx, ids)
}

func (f fakeRecipeWorkflows) WithinTx(ctx context.Context, fn func(q recipeWorkflowQueries) error) error {
	if f.withinTx == nil {
		return errors.New("WithinTx not implemented")
	}
	return f.withinTx(ctx, fn)
}

type fakeRecipeWorkflowQueries struct {
	createRecipe           func(ctx context.Context, arg sqlc.CreateRecipeParams) (sqlc.Recipe, error)
	createRecipeIngredient func(ctx context.Context, arg sqlc.CreateRecipeIngredientParams) error
	createRecipeStep       func(ctx context.Context, arg sqlc.CreateRecipeStepParams) error
	createRecipeTag        func(ctx context.Context, arg sqlc.CreateRecipeTagParams) error
	getRecipeDeletedAtByID func(ctx context.Context, id pgtype.UUID) (pgtype.Timestamptz, error)
	updateRecipeByID       func(ctx context.Context, arg sqlc.UpdateRecipeByIDParams) (sqlc.Recipe, error)
	deleteIngredientsByID  func(ctx context.Context, recipeID pgtype.UUID) error
	deleteStepsByID        func(ctx context.Context, recipeID pgtype.UUID) error
	deleteTagsByID         func(ctx context.Context, recipeID pgtype.UUID) error
}

func (f fakeRecipeWorkflowQueries) CreateRecipe(ctx context.Context, arg sqlc.CreateRecipeParams) (sqlc.Recipe, error) {
	if f.createRecipe == nil {
		return sqlc.Recipe{}, errors.New("CreateRecipe not implemented")
	}
	return f.createRecipe(ctx, arg)
}

func (f fakeRecipeWorkflowQueries) CreateRecipeIngredient(ctx context.Context, arg sqlc.CreateRecipeIngredientParams) error {
	if f.createRecipeIngredient == nil {
		return errors.New("CreateRecipeIngredient not implemented")
	}
	return f.createRecipeIngredient(ctx, arg)
}

func (f fakeRecipeWorkflowQueries) CreateRecipeStep(ctx context.Context, arg sqlc.CreateRecipeStepParams) error {
	if f.createRecipeStep == nil {
		return errors.New("CreateRecipeStep not implemented")
	}
	return f.createRecipeStep(ctx, arg)
}

func (f fakeRecipeWorkflowQueries) CreateRecipeTag(ctx context.Context, arg sqlc.CreateRecipeTagParams) error {
	if f.createRecipeTag == nil {
		return errors.New("CreateRecipeTag not implemented")
	}
	return f.createRecipeTag(ctx, arg)
}

func (f fakeRecipeWorkflowQueries) GetRecipeDeletedAtByID(ctx context.Context, id pgtype.UUID) (pgtype.Timestamptz, error) {
	if f.getRecipeDeletedAtByID == nil {
		return pgtype.Timestamptz{}, errors.New("GetRecipeDeletedAtByID not implemented")
	}
	return f.getRecipeDeletedAtByID(ctx, id)
}

func (f fakeRecipeWorkflowQueries) UpdateRecipeByID(ctx context.Context, arg sqlc.UpdateRecipeByIDParams) (sqlc.Recipe, error) {
	if f.updateRecipeByID == nil {
		return sqlc.Recipe{}, errors.New("UpdateRecipeByID not implemented")
	}
	return f.updateRecipeByID(ctx, arg)
}

func (f fakeRecipeWorkflowQueries) DeleteRecipeIngredientsByRecipeID(ctx context.Context, recipeID pgtype.UUID) error {
	if f.deleteIngredientsByID == nil {
		return errors.New("DeleteRecipeIngredientsByRecipeID not implemented")
	}
	return f.deleteIngredientsByID(ctx, recipeID)
}

func (f fakeRecipeWorkflowQueries) DeleteRecipeStepsByRecipeID(ctx context.Context, recipeID pgtype.UUID) error {
	if f.deleteStepsByID == nil {
		return errors.New("DeleteRecipeStepsByRecipeID not implemented")
	}
	return f.deleteStepsByID(ctx, recipeID)
}

func (f fakeRecipeWorkflowQueries) DeleteRecipeTagsByRecipeID(ctx context.Context, recipeID pgtype.UUID) error {
	if f.deleteTagsByID == nil {
		return errors.New("DeleteRecipeTagsByRecipeID not implemented")
	}
	return f.deleteTagsByID(ctx, recipeID)
}

const recipeBookIDField = "recipe_book_id"

func validCreateRecipeRequest() createRecipeRequest {
	return createRecipeRequest{
		Title:            "Tacos",
		Servings:         2,
		PrepTimeMinutes:  5,
		TotalTimeMinutes: 10,
		Ingredients: []recipeIngredientRequest{
			{Position: 1, Item: "Tortillas"},
		},
		Steps: []recipeStepRequest{
			{StepNumber: 1, Instruction: "Assemble."},
		},
	}
}

func TestCreateRecipeUsecase_ValidationErrors(t *testing.T) {
	t.Parallel()

	actorID := pgtype.UUID{Bytes: uuid.New(), Valid: true}

	t.Run("invalid recipe book id", func(t *testing.T) {
		t.Parallel()

		req := validCreateRecipeRequest()
		invalid := "not-a-uuid"
		req.RecipeBookID = &invalid

		workflows := fakeRecipeWorkflows{
			countTagsByIDs: func(ctx context.Context, ids []pgtype.UUID) (int32, error) {
				t.Fatalf("CountTagsByIDs should not be called")
				return 0, nil
			},
			withinTx: func(ctx context.Context, fn func(q recipeWorkflowQueries) error) error {
				t.Fatalf("WithinTx should not be called")
				return nil
			},
		}

		_, err := createRecipeUsecase(context.Background(), workflows, actorID, req)
		var v *recipeValidationError
		if !errors.As(err, &v) {
			t.Fatalf("expected *recipeValidationError, got %T (%v)", err, err)
		}
		if len(v.FieldErrors) != 1 || v.FieldErrors[0].Field != recipeBookIDField {
			t.Fatalf("unexpected field errors: %#v", v.FieldErrors)
		}
	})

	t.Run("invalid tag id", func(t *testing.T) {
		t.Parallel()

		req := validCreateRecipeRequest()
		req.TagIDs = []string{"not-a-uuid"}

		workflows := fakeRecipeWorkflows{
			countTagsByIDs: func(ctx context.Context, ids []pgtype.UUID) (int32, error) {
				t.Fatalf("CountTagsByIDs should not be called")
				return 0, nil
			},
			withinTx: func(ctx context.Context, fn func(q recipeWorkflowQueries) error) error {
				t.Fatalf("WithinTx should not be called")
				return nil
			},
		}

		_, err := createRecipeUsecase(context.Background(), workflows, actorID, req)
		var v *recipeValidationError
		if !errors.As(err, &v) {
			t.Fatalf("expected *recipeValidationError, got %T (%v)", err, err)
		}
		if len(v.FieldErrors) != 1 || v.FieldErrors[0].Field != "tag_ids" {
			t.Fatalf("unexpected field errors: %#v", v.FieldErrors)
		}
	})

	t.Run("tags do not exist", func(t *testing.T) {
		t.Parallel()

		req := validCreateRecipeRequest()
		req.TagIDs = []string{uuid.NewString()}

		workflows := fakeRecipeWorkflows{
			countTagsByIDs: func(ctx context.Context, ids []pgtype.UUID) (int32, error) {
				return 0, nil
			},
			withinTx: func(ctx context.Context, fn func(q recipeWorkflowQueries) error) error {
				t.Fatalf("WithinTx should not be called")
				return nil
			},
		}

		_, err := createRecipeUsecase(context.Background(), workflows, actorID, req)
		var v *recipeValidationError
		if !errors.As(err, &v) {
			t.Fatalf("expected *recipeValidationError, got %T (%v)", err, err)
		}
		if len(v.FieldErrors) != 1 || v.FieldErrors[0].Field != "tag_ids" {
			t.Fatalf("unexpected field errors: %#v", v.FieldErrors)
		}
	})

	t.Run("recipe book fk missing maps to validation", func(t *testing.T) {
		t.Parallel()

		req := validCreateRecipeRequest()
		bookID := uuid.NewString()
		req.RecipeBookID = &bookID

		workflows := fakeRecipeWorkflows{
			countTagsByIDs: func(ctx context.Context, ids []pgtype.UUID) (int32, error) {
				return 0, nil
			},
			withinTx: func(ctx context.Context, fn func(q recipeWorkflowQueries) error) error {
				return fn(fakeRecipeWorkflowQueries{
					createRecipe: func(ctx context.Context, arg sqlc.CreateRecipeParams) (sqlc.Recipe, error) {
						return sqlc.Recipe{}, &pgconn.PgError{Code: "23503"}
					},
				})
			},
		}

		_, err := createRecipeUsecase(context.Background(), workflows, actorID, req)
		var v *recipeValidationError
		if !errors.As(err, &v) {
			t.Fatalf("expected *recipeValidationError, got %T (%v)", err, err)
		}
		if len(v.FieldErrors) != 1 || v.FieldErrors[0].Field != recipeBookIDField {
			t.Fatalf("unexpected field errors: %#v", v.FieldErrors)
		}
	})
}

func TestUpdateRecipeUsecase_StateErrors(t *testing.T) {
	t.Parallel()

	actorID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	recipeID := pgtype.UUID{Bytes: uuid.New(), Valid: true}

	t.Run("missing recipe returns not found", func(t *testing.T) {
		t.Parallel()

		workflows := fakeRecipeWorkflows{
			countTagsByIDs: func(ctx context.Context, ids []pgtype.UUID) (int32, error) {
				return 0, nil
			},
			withinTx: func(ctx context.Context, fn func(q recipeWorkflowQueries) error) error {
				return fn(fakeRecipeWorkflowQueries{
					getRecipeDeletedAtByID: func(ctx context.Context, id pgtype.UUID) (pgtype.Timestamptz, error) {
						return pgtype.Timestamptz{}, pgx.ErrNoRows
					},
				})
			},
		}

		err := updateRecipeUsecase(context.Background(), workflows, actorID, recipeID, validCreateRecipeRequest())
		var nf *recipeNotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("expected *recipeNotFoundError, got %T (%v)", err, err)
		}
	})

	t.Run("deleted recipe returns conflict", func(t *testing.T) {
		t.Parallel()

		workflows := fakeRecipeWorkflows{
			countTagsByIDs: func(ctx context.Context, ids []pgtype.UUID) (int32, error) {
				return 0, nil
			},
			withinTx: func(ctx context.Context, fn func(q recipeWorkflowQueries) error) error {
				return fn(fakeRecipeWorkflowQueries{
					getRecipeDeletedAtByID: func(ctx context.Context, id pgtype.UUID) (pgtype.Timestamptz, error) {
						return pgtype.Timestamptz{Time: time.Now(), Valid: true}, nil
					},
				})
			},
		}

		err := updateRecipeUsecase(context.Background(), workflows, actorID, recipeID, validCreateRecipeRequest())
		var cf *recipeConflictError
		if !errors.As(err, &cf) {
			t.Fatalf("expected *recipeConflictError, got %T (%v)", err, err)
		}
	})

	t.Run("recipe book fk missing maps to validation", func(t *testing.T) {
		t.Parallel()

		req := validCreateRecipeRequest()
		bookID := uuid.NewString()
		req.RecipeBookID = &bookID

		workflows := fakeRecipeWorkflows{
			countTagsByIDs: func(ctx context.Context, ids []pgtype.UUID) (int32, error) {
				return 0, nil
			},
			withinTx: func(ctx context.Context, fn func(q recipeWorkflowQueries) error) error {
				return fn(fakeRecipeWorkflowQueries{
					getRecipeDeletedAtByID: func(ctx context.Context, id pgtype.UUID) (pgtype.Timestamptz, error) {
						return pgtype.Timestamptz{}, nil
					},
					updateRecipeByID: func(ctx context.Context, arg sqlc.UpdateRecipeByIDParams) (sqlc.Recipe, error) {
						return sqlc.Recipe{}, &pgconn.PgError{Code: "23503"}
					},
				})
			},
		}

		err := updateRecipeUsecase(context.Background(), workflows, actorID, recipeID, req)
		var v *recipeValidationError
		if !errors.As(err, &v) {
			t.Fatalf("expected *recipeValidationError, got %T (%v)", err, err)
		}
		if len(v.FieldErrors) != 1 || v.FieldErrors[0].Field != recipeBookIDField {
			t.Fatalf("unexpected field errors: %#v", v.FieldErrors)
		}
	})
}

func TestMapRecipeUsecaseError(t *testing.T) {
	t.Parallel()

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		err := mapRecipeUsecaseError(&recipeValidationError{
			FieldErrors: []response.FieldError{{Field: "tag_ids", Message: "invalid id"}},
		})
		apiErr, ok := asAPIError(err)
		if !ok {
			t.Fatalf("expected apiError, got %T (%v)", err, err)
		}
		if apiErr.kind != apiErrorValidation {
			t.Fatalf("expected kind %q, got %q", apiErrorValidation, apiErr.kind)
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		err := mapRecipeUsecaseError(&recipeNotFoundError{})
		apiErr, ok := asAPIError(err)
		if !ok {
			t.Fatalf("expected apiError, got %T (%v)", err, err)
		}
		if apiErr.kind != apiErrorNotFound {
			t.Fatalf("expected kind %q, got %q", apiErrorNotFound, apiErr.kind)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		t.Parallel()

		err := mapRecipeUsecaseError(&recipeConflictError{Message: "conflict"})
		apiErr, ok := asAPIError(err)
		if !ok {
			t.Fatalf("expected apiError, got %T (%v)", err, err)
		}
		if apiErr.kind != apiErrorConflict {
			t.Fatalf("expected kind %q, got %q", apiErrorConflict, apiErr.kind)
		}
	})
}
