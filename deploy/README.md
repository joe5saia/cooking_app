# Production deployment (skeleton)

This directory contains a production-oriented Docker Compose + Caddy skeleton aligned with `personal-recipe-app-spec.md`.

## Prereqs

- Docker + Docker Compose v2 on the host (Ubuntu recommended).
- A domain name for TLS (DNS-01 challenge recommended for LAN-only deployments).
- A Caddy build that includes your DNS provider module (this repo includes `deploy/Dockerfile.caddy`).

## Files

- `deploy/compose.yaml`: production-oriented stack (Postgres + API + Caddy).
- `deploy/Caddyfile`: serves the built frontend and proxies `/api/*` to the backend.

## Environment variables

Create a `.env` in the repo root (or export env vars before running compose):

- `COOKING_APP_DOMAIN` (required): the site domain used by Caddy (example: `recipes.example.com`).
- `POSTGRES_PASSWORD` (required): Postgres user password.
- `LOG_LEVEL` (optional): backend log level (default `info`).
- `CADDY_DNS_MODULE` (optional): DNS provider module for DNS-01 (default `github.com/caddy-dns/cloudflare`).

DNS-01 variables depend on your DNS provider and Caddy DNS plugin. Example for Cloudflare:

- `CLOUDFLARE_API_TOKEN`

## Build the frontend

Create a production build at `frontend/dist`:

```bash
make frontend-build
```

## Run

```bash
docker compose -f deploy/compose.yaml up -d --build
```

## Migrations

The production stack does not run migrations automatically unless you use `deploy/deploy.sh`.

To run migrations manually on the server (assuming you deployed to `~/apps/cooking_app`):

```bash
ssh kittenserver 'cd ~/apps/cooking_app && set -a && . ./.env && set +a && docker run --rm --network deploy_internal -v "$PWD/backend:/src" -w /src golang:1.25 go run github.com/pressly/goose/v3/cmd/goose@v3.26.0 -dir ./migrations postgres "postgres://cooking_app:${POSTGRES_PASSWORD}@db:5432/cooking_app?sslmode=disable" up'
```

## Create first user

To create the first user on a fresh database:

```bash
ssh kittenserver 'cd ~/apps/cooking_app && set -a && . ./.env && set +a && docker run --rm --network deploy_internal -v "$PWD/backend:/src" -w /src -e DATABASE_URL="postgres://cooking_app:${POSTGRES_PASSWORD}@db:5432/cooking_app?sslmode=disable" golang:1.25 go run ./cmd/cooking_app bootstrap-user --username admin --password "CHOOSE_A_PASSWORD" --display-name "Admin"'
```

## Backups

`deploy/backup-postgres.sh` is a sample `pg_dump` backup script intended to run on the host.

```bash
chmod +x deploy/backup-postgres.sh
./deploy/backup-postgres.sh
```

Retention:

- Default keeps the last 14 backups (`KEEP_DAILY=14`).
- Override via `KEEP_DAILY=30 ./deploy/backup-postgres.sh`.
- For a daily+weekly strategy, keep daily backups short (e.g., 14) and separately copy one backup per week to a `weekly/` folder with its own retention.

## Notes

- Only Caddy ports 80/443 are exposed. Postgres is internal.
- The backend is configured with `SESSION_COOKIE_SECURE=true` for HTTPS-only cookies.
