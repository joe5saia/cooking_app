#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: deploy/deploy.sh [--host HOST] [--ip IP] [--remote-dir DIR] [--lan-https] [--lan-https-redirect] [--reset-volumes]

Deploys the production stack in deploy/ to an Ubuntu server over SSH and verifies it via curl.

Defaults:
  HOST:       kittenserver
  IP:         192.168.88.15
  REMOTE_DIR: ~/apps/cooking_app

Environment variables (optional):
  POSTGRES_PASSWORD       Postgres password (auto-generated if unset)
  COOKING_APP_DOMAIN      Caddy site address (default :80 for HTTP on LAN)
  COOKING_APP_CADDYFILE   Caddyfile to use (default: Caddyfile; LAN HTTPS: Caddyfile.lan)
  COOKING_APP_LAN_IP      Server LAN IP for `Caddyfile.lan` (default: --ip)
  COOKING_APP_LAN_CADDYFILE  Caddyfile for LAN compose (default: Caddyfile.lan)
  COOKING_APP_BOOTSTRAP_USERNAME  First user username (default: admin)
  COOKING_APP_BOOTSTRAP_PASSWORD  First user password (default: sybil)
  COOKING_APP_BOOTSTRAP_DISPLAY_NAME  First user display name (default: Admin)
  SESSION_COOKIE_SECURE   true/false (default: true with --lan-https; otherwise false when COOKING_APP_DOMAIN=:80, else true)
  LOG_LEVEL               Backend log level (default info)
EOF
}

SSH_HOST="${SSH_HOST:-kittenserver}"
SERVER_IP="${SERVER_IP:-192.168.88.15}"
REMOTE_DIR="${REMOTE_DIR:-~/apps/cooking_app}"
SSH_OPTS=(-o BatchMode=yes -o StrictHostKeyChecking=accept-new)
RESET_VOLUMES=0
LAN_HTTPS=0
LAN_HTTPS_REDIRECT=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)
      SSH_HOST="${2:?missing value for --host}"
      shift 2
      ;;
    --ip)
      SERVER_IP="${2:?missing value for --ip}"
      shift 2
      ;;
    --remote-dir)
      REMOTE_DIR="${2:?missing value for --remote-dir}"
      shift 2
      ;;
    --lan-https)
      LAN_HTTPS=1
      shift 1
      ;;
    --lan-https-redirect)
      LAN_HTTPS=1
      LAN_HTTPS_REDIRECT=1
      shift 1
      ;;
    --reset-volumes)
      RESET_VOLUMES=1
      shift 1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if ! command -v rsync >/dev/null 2>&1; then
  echo "Missing dependency: rsync" >&2
  exit 1
fi
if ! command -v ssh >/dev/null 2>&1; then
  echo "Missing dependency: ssh" >&2
  exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
  echo "Missing dependency: curl" >&2
  exit 1
fi

REMOTE_HOME="$(ssh "${SSH_OPTS[@]}" "$SSH_HOST" 'printf %s "$HOME"')"
REMOTE_DIR_RESOLVED="$REMOTE_DIR"
if [[ "$REMOTE_DIR_RESOLVED" == "~" ]]; then
  REMOTE_DIR_RESOLVED="$REMOTE_HOME"
elif [[ "$REMOTE_DIR_RESOLVED" == "~/"* ]]; then
  REMOTE_DIR_RESOLVED="$REMOTE_HOME/${REMOTE_DIR_RESOLVED#\~/}"
fi

POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-}"
if [[ -z "$POSTGRES_PASSWORD" && "$RESET_VOLUMES" -ne 1 ]]; then
  legacy_dir="$REMOTE_HOME/~/apps/cooking_app"
  if [[ "$REMOTE_DIR_RESOLVED" != "$legacy_dir" ]]; then
    ssh "${SSH_OPTS[@]}" "$SSH_HOST" \
      "if [[ -d '$legacy_dir' && ! -d '$REMOTE_DIR_RESOLVED' ]]; then mkdir -p '$(dirname "$REMOTE_DIR_RESOLVED")' && mv '$legacy_dir' '$REMOTE_DIR_RESOLVED'; fi" \
      >/dev/null 2>&1 || true
  fi

  existing_password="$(
    ssh "${SSH_OPTS[@]}" "$SSH_HOST" \
      "if [[ -f '$REMOTE_DIR_RESOLVED/.env' ]]; then grep -E '^POSTGRES_PASSWORD=' '$REMOTE_DIR_RESOLVED/.env' | head -n1 | cut -d= -f2-; fi" \
      2>/dev/null || true
  )"
  if [[ -n "${existing_password:-}" ]]; then
    if [[ ! "$existing_password" =~ ^[A-Za-z0-9]+$ ]]; then
      echo "Existing POSTGRES_PASSWORD in $SSH_HOST:$REMOTE_DIR_RESOLVED/.env contains URL-unsafe characters." >&2
      echo "Set a new URL-safe password and re-run with --reset-volumes (destructive) or provide POSTGRES_PASSWORD explicitly." >&2
      exit 1
    fi
    POSTGRES_PASSWORD="$existing_password"
    echo "Using existing POSTGRES_PASSWORD from $SSH_HOST:$REMOTE_DIR_RESOLVED/.env" >&2
  fi
fi

if [[ -z "$POSTGRES_PASSWORD" ]]; then
  if command -v openssl >/dev/null 2>&1; then
    POSTGRES_PASSWORD="$(openssl rand -hex 24 | tr -d '\n')"
  else
    POSTGRES_PASSWORD="$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32)"
  fi
  echo "Generated POSTGRES_PASSWORD (saved on server): $POSTGRES_PASSWORD" >&2
fi

COOKING_APP_DOMAIN="${COOKING_APP_DOMAIN:-:80}"
COOKING_APP_CADDYFILE="${COOKING_APP_CADDYFILE:-Caddyfile}"
COOKING_APP_LAN_IP="${COOKING_APP_LAN_IP:-$SERVER_IP}"
COOKING_APP_LAN_CADDYFILE="${COOKING_APP_LAN_CADDYFILE:-Caddyfile.lan}"
COOKING_APP_BOOTSTRAP_USERNAME="${COOKING_APP_BOOTSTRAP_USERNAME:-admin}"
COOKING_APP_BOOTSTRAP_PASSWORD="${COOKING_APP_BOOTSTRAP_PASSWORD:-sybil}"
COOKING_APP_BOOTSTRAP_DISPLAY_NAME="${COOKING_APP_BOOTSTRAP_DISPLAY_NAME:-Admin}"

compose_file_flags="-f deploy/compose.yaml"
if [[ "$LAN_HTTPS" -eq 1 ]]; then
  compose_file_flags="-f deploy/compose.lan.yaml"
fi

if [[ "$LAN_HTTPS_REDIRECT" -eq 1 ]]; then
  COOKING_APP_LAN_CADDYFILE="Caddyfile.lan.redirect"
fi
SESSION_COOKIE_SECURE="${SESSION_COOKIE_SECURE:-}"
if [[ -z "$SESSION_COOKIE_SECURE" ]]; then
  if [[ "$LAN_HTTPS" -eq 1 ]]; then
    SESSION_COOKIE_SECURE="true"
  elif [[ "$COOKING_APP_DOMAIN" == ":80" ]]; then
    SESSION_COOKIE_SECURE="false"
  else
    SESSION_COOKIE_SECURE="true"
  fi
fi
LOG_LEVEL="${LOG_LEVEL:-info}"

echo "Building frontend (frontend/dist)..." >&2
make frontend-build

echo "Connecting to $SSH_HOST and preparing $REMOTE_DIR_RESOLVED..." >&2
ssh "${SSH_OPTS[@]}" "$SSH_HOST" "mkdir -p '$REMOTE_DIR_RESOLVED'"

echo "Syncing repo to $SSH_HOST:$REMOTE_DIR_RESOLVED ..." >&2
if ssh "${SSH_OPTS[@]}" "$SSH_HOST" "command -v rsync >/dev/null 2>&1"; then
  rsync -az --delete \
    --exclude '.git/' \
    --exclude '.beads/' \
    --exclude 'backend/bin/' \
    --exclude 'backend/.air/' \
    --exclude 'frontend/node_modules/' \
    --exclude 'frontend/dist/.vite/' \
    --exclude '*.swp' \
    --exclude '._*' \
    --exclude '**/.DS_Store' \
    ./ "$SSH_HOST:$REMOTE_DIR_RESOLVED/"
else
  echo "Remote rsync not found; falling back to tar-over-ssh sync." >&2
  echo "Stopping running stack (prevents bind-mount invalidation during sync)..." >&2
  ssh "${SSH_OPTS[@]}" "$SSH_HOST" "cd '$REMOTE_DIR_RESOLVED' && docker compose --env-file .env $compose_file_flags down --remove-orphans" >/dev/null 2>&1 || true
  ssh "${SSH_OPTS[@]}" "$SSH_HOST" "find '$REMOTE_DIR_RESOLVED' -mindepth 1 -maxdepth 1 -exec rm -rf {} +"
  COPYFILE_DISABLE=1 COPY_EXTENDED_ATTRIBUTES_DISABLE=1 tar -czf - \
    --exclude='.git' \
    --exclude='.beads' \
    --exclude='backend/bin' \
    --exclude='backend/.air' \
    --exclude='frontend/node_modules' \
    --exclude='frontend/dist/.vite' \
    --exclude='*.swp' \
    --exclude='._*' \
    --exclude='.DS_Store' \
    . | ssh "${SSH_OPTS[@]}" "$SSH_HOST" "tar -xzf - -C '$REMOTE_DIR_RESOLVED'"
fi

echo "Writing remote env file..." >&2
ssh "${SSH_OPTS[@]}" "$SSH_HOST" "umask 077 && cat >'$REMOTE_DIR_RESOLVED/.env' <<'EOF'
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
COOKING_APP_DOMAIN=$COOKING_APP_DOMAIN
COOKING_APP_CADDYFILE=$COOKING_APP_CADDYFILE
COOKING_APP_LAN_IP=$COOKING_APP_LAN_IP
COOKING_APP_LAN_CADDYFILE=$COOKING_APP_LAN_CADDYFILE
SESSION_COOKIE_SECURE=$SESSION_COOKIE_SECURE
LOG_LEVEL=$LOG_LEVEL
EOF"

echo "Starting/Updating containers (docker compose)..." >&2
if [[ "$RESET_VOLUMES" -eq 1 ]]; then
  echo "Resetting docker compose volumes (destructive)..." >&2
  ssh "${SSH_OPTS[@]}" "$SSH_HOST" "cd '$REMOTE_DIR_RESOLVED' && docker compose --env-file .env $compose_file_flags down -v --remove-orphans" || true
fi

echo "Starting database..." >&2
ssh "${SSH_OPTS[@]}" "$SSH_HOST" "cd '$REMOTE_DIR_RESOLVED' && docker compose --env-file .env $compose_file_flags up -d --build db"

echo "Running migrations (goose)..." >&2
ssh "${SSH_OPTS[@]}" "$SSH_HOST" "cd '$REMOTE_DIR_RESOLVED' && set -a && . ./.env && set +a && for i in {1..30}; do docker run --rm --network deploy_internal -v \"\$PWD/backend:/src\" -w /src golang:1.25 go run github.com/pressly/goose/v3/cmd/goose@v3.26.0 -dir ./migrations postgres \"postgres://cooking_app:\${POSTGRES_PASSWORD}@db:5432/cooking_app?sslmode=disable\" up && exit 0; sleep 2; done; exit 1"

echo "Bootstrapping default user (if needed)..." >&2
bootstrap_args=(
  --host "$SSH_HOST"
  --remote-dir "$REMOTE_DIR_RESOLVED"
  --username "$COOKING_APP_BOOTSTRAP_USERNAME"
  --password "$COOKING_APP_BOOTSTRAP_PASSWORD"
  --display-name "$COOKING_APP_BOOTSTRAP_DISPLAY_NAME"
)
"$ROOT_DIR/deploy/bootstrap-user.sh" "${bootstrap_args[@]}"

echo "Starting API + Caddy..." >&2
ssh "${SSH_OPTS[@]}" "$SSH_HOST" "cd '$REMOTE_DIR_RESOLVED' && docker compose --env-file .env $compose_file_flags up -d --build api caddy"

echo "Waiting for HTTP health endpoint..." >&2
health_url="http://$SERVER_IP/api/v1/healthz"
healthy=0
for _ in {1..60}; do
  if curl -fsS "$health_url" >/dev/null 2>&1; then
    healthy=1
    break
  fi
  sleep 2
done

if [[ "$healthy" -ne 1 ]]; then
  echo "Health check failed: $health_url" >&2
  ssh "${SSH_OPTS[@]}" "$SSH_HOST" "cd '$REMOTE_DIR_RESOLVED' && docker compose --env-file .env $compose_file_flags ps -a" >&2 || true
  ssh "${SSH_OPTS[@]}" "$SSH_HOST" "cd '$REMOTE_DIR_RESOLVED' && docker compose --env-file .env $compose_file_flags logs --tail=200 db api caddy" >&2 || true
  exit 1
fi

echo "Health check:" >&2
curl -fsS "$health_url" >&2
echo >&2

echo "Deployed." >&2
echo "Visit: http://$SERVER_IP/"
if [[ "$LAN_HTTPS" -eq 1 ]]; then
  echo "Visit (requires trusting Caddy CA): https://$SERVER_IP/"
fi
