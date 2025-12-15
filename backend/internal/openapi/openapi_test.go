package openapi_test

import (
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestOpenAPI_IsValidAndHasRequiredPaths(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "openapi.yaml")

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatalf("load openapi: %v", err)
	}

	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("validate openapi: %v", err)
	}

	requiredPaths := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/logout",
		"/api/v1/auth/me",
		"/api/v1/tokens",
		"/api/v1/tokens/{id}",
		"/api/v1/users",
		"/api/v1/users/{id}/deactivate",
		"/api/v1/recipe-books",
		"/api/v1/recipe-books/{id}",
		"/api/v1/tags",
		"/api/v1/tags/{id}",
		"/api/v1/recipes",
		"/api/v1/recipes/{id}",
		"/api/v1/recipes/{id}/restore",
	}

	for _, p := range requiredPaths {
		if doc.Paths == nil || doc.Paths.Find(p) == nil {
			t.Fatalf("missing required path: %s", p)
		}
	}
}
