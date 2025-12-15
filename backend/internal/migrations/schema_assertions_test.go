package migrations_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func assertRegclassExists(ctx context.Context, t *testing.T, db *sql.DB, name string) {
	t.Helper()

	var exists bool
	if err := db.QueryRowContext(ctx, "select to_regclass($1) is not null", name).Scan(&exists); err != nil {
		t.Fatalf("check regclass %q: %v", name, err)
	}
	if !exists {
		t.Fatalf("expected regclass %q to exist", name)
	}
}

func assertColumnExists(ctx context.Context, t *testing.T, db *sql.DB, table, column string) {
	t.Helper()

	var exists bool
	if err := db.QueryRowContext(ctx, `
select exists(
  select 1
  from information_schema.columns
  where table_schema = 'public' and table_name = $1 and column_name = $2
)`, table, column).Scan(&exists); err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	if !exists {
		t.Fatalf("expected column %s.%s to exist", table, column)
	}
}

func assertColumnUDT(ctx context.Context, t *testing.T, db *sql.DB, table, column, udtName string) {
	t.Helper()

	var got string
	if err := db.QueryRowContext(ctx, `
select udt_name
from information_schema.columns
where table_schema = 'public' and table_name = $1 and column_name = $2
`, table, column).Scan(&got); err != nil {
		t.Fatalf("get column udt %s.%s: %v", table, column, err)
	}
	if got != udtName {
		t.Fatalf("column %s.%s udt=%q, want %q", table, column, got, udtName)
	}
}

func assertConstraintExists(ctx context.Context, t *testing.T, db *sql.DB, name string) {
	t.Helper()

	var exists bool
	if err := db.QueryRowContext(ctx, "select exists(select 1 from pg_constraint where conname = $1)", name).Scan(&exists); err != nil {
		t.Fatalf("check constraint %q: %v", name, err)
	}
	if !exists {
		t.Fatalf("expected constraint %q to exist", name)
	}
}

func assertFKHasOnDeleteCascade(ctx context.Context, t *testing.T, db *sql.DB, name string) {
	t.Helper()

	var def string
	if err := db.QueryRowContext(ctx, "select pg_get_constraintdef(oid) from pg_constraint where conname = $1", name).Scan(&def); err != nil {
		t.Fatalf("get fk def %q: %v", name, err)
	}
	if !strings.Contains(def, "ON DELETE CASCADE") {
		t.Fatalf("fk %q definition does not include ON DELETE CASCADE: %q", name, def)
	}
}
