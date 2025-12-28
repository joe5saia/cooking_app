package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/config"
)

func TestWriteTableRecipeListNextCursor(t *testing.T) {
	t.Parallel()

	nextCursor := "cursor-1"
	resp := client.RecipeListResponse{
		Items: []client.RecipeListItem{
			{
				ID:               testRecipeID,
				Title:            "Soup",
				Servings:         2,
				PrepTimeMinutes:  5,
				TotalTimeMinutes: 20,
				Tags:             []client.RecipeTag{},
				UpdatedAt:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		NextCursor: &nextCursor,
	}

	stdout := &bytes.Buffer{}
	exitCode := writeOutput(stdout, config.OutputTable, resp)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}
	if !strings.Contains(stdout.String(), "next_cursor=cursor-1") {
		t.Fatalf("expected next_cursor output, got %q", stdout.String())
	}
}

func TestHandleAPIErrorJSON(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdout: stdout,
		stderr: stderr,
	}

	err := &client.APIError{
		StatusCode: http.StatusForbidden,
		Problem: client.Problem{
			Code:    "forbidden",
			Message: "csrf missing",
			Details: []client.FieldError{
				{Field: "csrf", Message: "required"},
			},
		},
	}

	exitCode := app.handleAPIError(err)
	if exitCode != exitForbidden {
		t.Fatalf("exit code = %d, want %d", exitCode, exitForbidden)
	}

	var got apiErrorOutput
	if decodeErr := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&got); decodeErr != nil {
		t.Fatalf("decode output: %v", decodeErr)
	}
	if got.Error.Status != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", got.Error.Status, http.StatusForbidden)
	}
	if got.Error.Code != "forbidden" {
		t.Fatalf("code = %q, want %q", got.Error.Code, "forbidden")
	}
	if got.Error.Message != "csrf missing" {
		t.Fatalf("message = %q, want %q", got.Error.Message, "csrf missing")
	}
	if len(got.Error.Details) != 1 || got.Error.Details[0].Field != "csrf" {
		t.Fatalf("details = %#v, want csrf field", got.Error.Details)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestHandleAPIErrorTable(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdout: stdout,
		stderr: stderr,
	}

	err := &client.APIError{
		StatusCode: http.StatusConflict,
		Problem: client.Problem{
			Code:    "conflict",
			Message: "duplicate",
		},
	}

	exitCode := app.handleAPIError(err)
	if exitCode != exitConflict {
		t.Fatalf("exit code = %d, want %d", exitCode, exitConflict)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "conflict") {
		t.Fatalf("expected stderr to include conflict, got %q", stderr.String())
	}
}
