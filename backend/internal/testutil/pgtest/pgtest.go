package pgtest

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/saiaj/cooking_app/backend/internal/migrations"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Instance is a disposable Postgres instance for integration tests.
type Instance struct {
	DatabaseURL string
	container   testcontainers.Container
}

// Start boots a Postgres 18 container and returns its connection string.
// If Docker isn't available, the test is skipped unless `CI` is set.
func Start(ctx context.Context, t *testing.T) *Instance {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:18-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "app",
			"POSTGRES_PASSWORD": "app",
			"POSTGRES_DB":       "app",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("start postgres: %v", err)
		}
		t.Skipf("start postgres (docker not available?): %v", err)
	}

	t.Cleanup(func() {
		if termErr := container.Terminate(context.Background()); termErr != nil {
			t.Errorf("terminate container: %v", termErr)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}

	dbURL := fmt.Sprintf("postgres://app:app@%s:%s/app?sslmode=disable", host, port.Port())
	return &Instance{
		DatabaseURL: dbURL,
		container:   container,
	}
}

// OpenSQL opens a database/sql connection and registers a cleanup.
func (i *Instance) OpenSQL(ctx context.Context, t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("pgx", i.DatabaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("close db: %v", closeErr)
		}
	})

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	return db
}

// NewPool opens a pgxpool connection and registers a cleanup.
func (i *Instance) NewPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(ctx, i.DatabaseURL)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping pool: %v", err)
	}

	return pool
}

// MigrateUp applies all migrations (goose) to the given database connection.
func (i *Instance) MigrateUp(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}

	if err := goose.UpContext(ctx, db, migrations.Dir()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
}
