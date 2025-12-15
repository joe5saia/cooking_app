package migrations_test

import (
	"context"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

func TestGooseMigrationsApply(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)

	postgres.MigrateUp(ctx, t, db)

	var hasVersionTable bool
	if err := db.QueryRowContext(ctx, "select to_regclass('public.goose_db_version') is not null").Scan(&hasVersionTable); err != nil {
		t.Fatalf("check goose version table: %v", err)
	}
	if !hasVersionTable {
		t.Fatalf("expected goose_db_version table to exist")
	}

	var pgcrypto, citext, pgTrgm bool
	if err := db.QueryRowContext(ctx, `
select
  exists(select 1 from pg_extension where extname = 'pgcrypto') as pgcrypto,
  exists(select 1 from pg_extension where extname = 'citext') as citext,
  exists(select 1 from pg_extension where extname = 'pg_trgm') as pg_trgm
`).Scan(&pgcrypto, &citext, &pgTrgm); err != nil {
		t.Fatalf("check extensions: %v", err)
	}
	if !pgcrypto || !citext || !pgTrgm {
		t.Fatalf("expected extensions enabled; pgcrypto=%t citext=%t pg_trgm=%t", pgcrypto, citext, pgTrgm)
	}
}
