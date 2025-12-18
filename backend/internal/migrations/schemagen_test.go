package migrations_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/saiaj/cooking_app/backend/internal/migrations"
)

func TestGenerateSQLCSchemaFromGooseMigrations_MatchesCheckedInSchema(t *testing.T) {
	t.Parallel()

	migrationsDir := migrations.Dir()
	backendDir := filepath.Dir(migrationsDir)
	schemaPath := filepath.Join(backendDir, "internal", "db", "schema.sql")

	want, err := os.ReadFile(schemaPath) //nolint:gosec // schemaPath is derived from repo migrations dir
	if err != nil {
		t.Fatalf("read checked-in schema: %v", err)
	}

	got, err := migrations.GenerateSQLCSchemaFromGooseMigrations(migrationsDir)
	if err != nil {
		t.Fatalf("generate schema: %v", err)
	}

	if string(got) != string(want) {
		t.Fatalf("schema drift detected: run `make -C backend schema-generate` to update %s", schemaPath)
	}
}
