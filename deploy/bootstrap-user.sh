#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: deploy/bootstrap-user.sh --username USERNAME --password PASSWORD [--display-name NAME] [--host HOST] [--remote-dir DIR]

Bootstraps the first user by running the backend CLI on the deployed host.

Defaults:
  HOST:       kittenserver
  REMOTE_DIR: ~/apps/cooking_app
EOF
}

SSH_HOST="${SSH_HOST:-kittenserver}"
REMOTE_DIR="${REMOTE_DIR:-~/apps/cooking_app}"
USERNAME=""
PASSWORD=""
DISPLAY_NAME=""
SSH_OPTS=(-o BatchMode=yes -o StrictHostKeyChecking=accept-new)

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)
      SSH_HOST="${2:?missing value for --host}"
      shift 2
      ;;
    --remote-dir)
      REMOTE_DIR="${2:?missing value for --remote-dir}"
      shift 2
      ;;
    --username)
      USERNAME="${2:?missing value for --username}"
      shift 2
      ;;
    --password)
      PASSWORD="${2:?missing value for --password}"
      shift 2
      ;;
    --display-name)
      DISPLAY_NAME="${2:?missing value for --display-name}"
      shift 2
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

if [[ -z "$USERNAME" || -z "$PASSWORD" ]]; then
  echo "Missing required --username and/or --password." >&2
  usage >&2
  exit 2
fi

if ! command -v ssh >/dev/null 2>&1; then
  echo "Missing dependency: ssh" >&2
  exit 1
fi
if ! command -v base64 >/dev/null 2>&1; then
  echo "Missing dependency: base64" >&2
  exit 1
fi

REMOTE_HOME="$(ssh "${SSH_OPTS[@]}" "$SSH_HOST" 'printf %s "$HOME"')"
REMOTE_DIR_RESOLVED="$REMOTE_DIR"
if [[ "$REMOTE_DIR_RESOLVED" == "~" ]]; then
  REMOTE_DIR_RESOLVED="$REMOTE_HOME"
elif [[ "$REMOTE_DIR_RESOLVED" == "~/"* ]]; then
  REMOTE_DIR_RESOLVED="$REMOTE_HOME/${REMOTE_DIR_RESOLVED#\~/}"
fi

# Base64 keeps the SSH command line simple without leaking special characters.
username_b64="$(printf "%s" "$USERNAME" | base64 | tr -d '\n')"
password_b64="$(printf "%s" "$PASSWORD" | base64 | tr -d '\n')"
display_name_b64="$(printf "%s" "$DISPLAY_NAME" | base64 | tr -d '\n')"

echo "Bootstrapping user on $SSH_HOST:$REMOTE_DIR_RESOLVED ..." >&2

set +e
ssh_output="$(
  ssh "${SSH_OPTS[@]}" "$SSH_HOST" \
    "USERNAME_B64='$username_b64' PASSWORD_B64='$password_b64' DISPLAY_NAME_B64='$display_name_b64' REMOTE_DIR='$REMOTE_DIR_RESOLVED' bash -lc 'set -euo pipefail
      if ! command -v docker >/dev/null 2>&1; then
        echo \"Missing dependency: docker\" >&2
        exit 1
      fi
      if ! command -v base64 >/dev/null 2>&1; then
        echo \"Missing dependency: base64\" >&2
        exit 1
      fi

      username=\"\$(printf \"%s\" \"\$USERNAME_B64\" | base64 --decode)\"
      password=\"\$(printf \"%s\" \"\$PASSWORD_B64\" | base64 --decode)\"
      display_name=\"\$(printf \"%s\" \"\$DISPLAY_NAME_B64\" | base64 --decode)\"

      if [[ -z \"\$username\" || -z \"\$password\" ]]; then
        echo \"Username and password are required.\" >&2
        exit 2
      fi

      cd \"\$REMOTE_DIR\"
      if [[ ! -f ./.env ]]; then
        echo \"Missing .env at \$REMOTE_DIR/.env\" >&2
        exit 1
      fi

      set -a
      . ./.env
      set +a

      if [[ -z \"\${POSTGRES_PASSWORD:-}\" ]]; then
        echo \"POSTGRES_PASSWORD is required in .env\" >&2
        exit 1
      fi

      database_url=\"postgres://cooking_app:\${POSTGRES_PASSWORD}@db:5432/cooking_app?sslmode=disable\"
      args=(--username \"\$username\" --password \"\$password\")
      if [[ -n \"\$display_name\" ]]; then
        args+=(--display-name \"\$display_name\")
      fi

      docker run --rm --network deploy_internal -v \"\$PWD/backend:/src\" -w /src -e DATABASE_URL=\"\$database_url\" golang:1.25 go run ./cmd/cli bootstrap-user \"\${args[@]}\"
    '" 2>&1
)"
ssh_status=$?
set -e

if [[ "$ssh_status" -ne 0 ]]; then
  if printf "%s" "$ssh_output" | grep -Fq "bootstrap refused: users table is not empty"; then
    printf "%s\n" "$ssh_output" >&2
    exit 0
  fi
  printf "%s\n" "$ssh_output" >&2
  exit "$ssh_status"
fi

if [[ -n "$ssh_output" ]]; then
  printf "%s\n" "$ssh_output"
fi
