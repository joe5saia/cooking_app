# Production deployment (skeleton)

This directory contains a production-oriented Docker Compose + Caddy skeleton aligned with `personal-recipe-app-spec.md`.

## Prereqs

- Docker + Docker Compose v2 on the host (Ubuntu recommended).
- A domain name for public TLS (DNS-01 challenge recommended if you want TLS without exposing port 80).
- For LAN-only HTTPS without a domain, use Caddy's internal CA (`deploy/Caddyfile.lan`).
- A Caddy build that includes your DNS provider module (this repo includes `deploy/Dockerfile.caddy`).

## Files

- `deploy/compose.yaml`: production-oriented stack (Postgres + API + Caddy).
- `deploy/compose.lan.yaml`: LAN HTTPS stack variant (Caddy on host network).
- `deploy/Caddyfile`: domain-based config (typical public TLS).
- `deploy/Caddyfile.lan`: LAN-friendly config that serves both HTTP (:80) and HTTPS (:443) without redirect.

## Environment variables

Create a `.env` in the repo root (or export env vars before running compose):

- `COOKING_APP_CADDYFILE` (optional): which Caddyfile to use with `deploy/compose.yaml` (default `Caddyfile`).
- `COOKING_APP_DOMAIN` (optional): the site address used by `deploy/Caddyfile` (example: `recipes.example.com`; default `:80`).
- `COOKING_APP_LAN_IP` (required for `deploy/compose.lan.yaml`): server LAN IP used for IP-based HTTPS certs (example: `192.168.88.15`).
- `COOKING_APP_LAN_CADDYFILE` (optional for `deploy/compose.lan.yaml`): which LAN caddyfile to use (default `Caddyfile.lan`).
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

`deploy/deploy.sh` defaults to the LAN HTTPS stack (`deploy/compose.lan.yaml`). Pass `--no-lan-https` to use the standard compose file (`deploy/compose.yaml`).

### LAN HTTPS (optional, no redirect)

To serve both `http://<server-ip>/` and `https://<server-ip>/` (no forced redirect) using Caddy internal TLS:

```bash
export COOKING_APP_LAN_IP=192.168.88.15
docker compose -f deploy/compose.lan.yaml up -d --build
```

Notes:

- `deploy/compose.lan.yaml` runs Caddy with `network_mode: host` so `https://<lan-ip>/` works reliably (clients often omit SNI for IP URLs).
- `network_mode: host` is intended for Linux hosts (e.g. Ubuntu). Docker Desktop (macOS/Windows) does not support this workflow.
- `https://...` uses `tls internal`, so clients must trust Caddy's internal CA to avoid browser warnings.

To export and trust the CA on your devices, see `deploy/TRUST_CADDY_CA.md` (or run `deploy/trust-caddy-ca.sh`).

### LAN HTTPS with redirect (optional)

If you prefer to force HTTPS (HTTP redirects to HTTPS):

```bash
export COOKING_APP_LAN_IP=192.168.88.15
export COOKING_APP_LAN_CADDYFILE=Caddyfile.lan.redirect
docker compose -f deploy/compose.lan.yaml up -d --build
```

### LAN HTTPS with a hostname (recommended when possible)

Using a stable local hostname avoids some of the friction of IP-based HTTPS (clients send SNI for hostnames).

Recommended approaches (choose one):

- Router DNS: add a DNS record (or DHCP reservation + hostname) that maps `cooking-app.home.arpa` (or `cooking-app.lan`) to your server IP.
- mDNS: some networks support `.local` names (example: `cooking-app.local`), but support varies by OS/router.

If you only want HTTP (no HTTPS), you can also set `COOKING_APP_DOMAIN` to the hostname + `:80`:

```bash
export COOKING_APP_DOMAIN=cooking-app.home.arpa:80
docker compose -f deploy/compose.yaml up -d --build
```

Then run the regular production compose file with the hostname-based Caddyfile:

```bash
export COOKING_APP_LAN_HOST=cooking-app.home.arpa
export COOKING_APP_CADDYFILE=Caddyfile.lan.hostname
docker compose -f deploy/compose.yaml up -d --build
```

If you also want HTTP->HTTPS redirect, use:

```bash
export COOKING_APP_LAN_HOST=cooking-app.home.arpa
export COOKING_APP_CADDYFILE=Caddyfile.lan.hostname.redirect
docker compose -f deploy/compose.yaml up -d --build
```

## Migrations

The production stack does not run migrations automatically unless you use `deploy/deploy.sh`.

To run migrations manually on the server (assuming you deployed to `~/apps/cooking_app`):

```bash
ssh kittenserver 'cd ~/apps/cooking_app && set -a && . ./.env && set +a && docker run --rm --network deploy_internal -v "$PWD/backend:/src" -w /src golang:1.25 go run github.com/pressly/goose/v3/cmd/goose@v3.26.0 -dir ./migrations postgres "postgres://cooking_app:${POSTGRES_PASSWORD}@db:5432/cooking_app?sslmode=disable" up'
```

## Create first user

If you use `deploy/deploy.sh`, it automatically bootstraps a default user (username `admin`, password `sybil`, display name `Admin`) when the database is empty. You can override those defaults by setting:

- `COOKING_APP_BOOTSTRAP_USERNAME`
- `COOKING_APP_BOOTSTRAP_PASSWORD`
- `COOKING_APP_BOOTSTRAP_DISPLAY_NAME`

To create the first user manually on a fresh database:

```bash
ssh kittenserver 'cd ~/apps/cooking_app && set -a && . ./.env && set +a && docker run --rm --network deploy_internal -v "$PWD/backend:/src" -w /src -e DATABASE_URL="postgres://cooking_app:${POSTGRES_PASSWORD}@db:5432/cooking_app?sslmode=disable" golang:1.25 go run ./cmd/cli bootstrap-user --username admin --password "CHOOSE_A_PASSWORD" --display-name "Admin"'
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
- `SESSION_COOKIE_SECURE` should be `true` when you use HTTPS. If you enable `Caddyfile.lan`, HTTP will still serve without redirect, but secure cookies will not be sent over HTTP.
- LAN HTTP-only (default `COOKING_APP_DOMAIN=:80` with `Caddyfile`): the app is served over `http://<server-ip>/`.
