# Migrations

This backend uses `goose` (SQL migrations) and connects using `DATABASE_URL` (PostgreSQL 18).

## Quick start (dev stack)

1. Start Postgres:
   - from repo root: `make dev-up`
2. Run migrations from your host:
   - `DATABASE_URL='postgres://app:app@localhost:5432/app?sslmode=disable' make -C backend migrate-up`

## Make targets

- Show goose version: `make -C backend migrate-version`
- Migration status: `DATABASE_URL=... make -C backend migrate-status`
- Apply migrations: `DATABASE_URL=... make -C backend migrate-up`
- Roll back 1 migration: `DATABASE_URL=... make -C backend migrate-down`
- Create a new SQL migration:
  - `make -C backend migrate-create name=add_recipes_table`

## sqlc schema source-of-truth

Migrations are the single source-of-truth for the database schema.

- `backend/internal/db/schema.sql` is generated from the Goose migrations' `Up` sections.
- Update it after changing migrations: `make -C backend schema-generate`
- CI enforces drift checks via `make -C backend schema-check` (runs as part of `make backend-ci`).
