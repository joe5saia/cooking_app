#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: deploy/trust-caddy-ca.sh [--host HOST] [--remote-dir DIR] [--output PATH] [--install-macos]

Exports Caddy’s internal root CA certificate from a deployed LAN HTTPS stack (deploy/compose.lan.yaml).

Defaults:
  HOST:       kittenserver
  REMOTE_DIR: ~/apps/cooking_app
  OUTPUT:     /tmp/cooking_app-caddy-local-root.crt

Options:
  --install-macos  Installs the exported CA into macOS System Keychain (requires sudo).
EOF
}

SSH_HOST="${SSH_HOST:-kittenserver}"
REMOTE_DIR="${REMOTE_DIR:-~/apps/cooking_app}"
OUTPUT_PATH="${OUTPUT_PATH:-/tmp/cooking_app-caddy-local-root.crt}"
INSTALL_MACOS=0
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
    --output)
      OUTPUT_PATH="${2:?missing value for --output}"
      shift 2
      ;;
    --install-macos)
      INSTALL_MACOS=1
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

if ! command -v ssh >/dev/null 2>&1; then
  echo "Missing dependency: ssh" >&2
  exit 1
fi
if ! command -v openssl >/dev/null 2>&1; then
  echo "Missing dependency: openssl" >&2
  exit 1
fi

REMOTE_HOME="$(ssh "${SSH_OPTS[@]}" "$SSH_HOST" 'printf %s "$HOME"')"
REMOTE_DIR_RESOLVED="$REMOTE_DIR"
if [[ "$REMOTE_DIR_RESOLVED" == "~" ]]; then
  REMOTE_DIR_RESOLVED="$REMOTE_HOME"
elif [[ "$REMOTE_DIR_RESOLVED" == "~/"* ]]; then
  REMOTE_DIR_RESOLVED="$REMOTE_HOME/${REMOTE_DIR_RESOLVED#\~/}"
fi

tmp_out="${OUTPUT_PATH}.tmp.$$"
mkdir -p "$(dirname "$OUTPUT_PATH")"

echo "Exporting Caddy CA from $SSH_HOST:$REMOTE_DIR_RESOLVED ..." >&2
ssh "${SSH_OPTS[@]}" "$SSH_HOST" \
  "cd '$REMOTE_DIR_RESOLVED' && docker compose --env-file .env -f deploy/compose.lan.yaml exec -T caddy sh -c 'cat /data/caddy/pki/authorities/local/root.crt'" \
  >"$tmp_out"

if [[ ! -s "$tmp_out" ]]; then
  echo "Export failed: output is empty." >&2
  echo "Ensure the LAN HTTPS stack is running and has generated the CA (deploy/compose.lan.yaml + deploy/Caddyfile.lan)." >&2
  rm -f "$tmp_out"
  exit 1
fi

if ! openssl x509 -in "$tmp_out" -noout >/dev/null 2>&1; then
  echo "Export failed: output does not look like an X.509 certificate." >&2
  rm -f "$tmp_out"
  exit 1
fi

mv -f "$tmp_out" "$OUTPUT_PATH"

echo "Wrote: $OUTPUT_PATH" >&2
echo "Certificate:" >&2
openssl x509 -in "$OUTPUT_PATH" -noout -subject -issuer -dates -fingerprint -sha256 >&2

if [[ "$INSTALL_MACOS" -eq 1 ]]; then
  if [[ "$(uname -s)" != "Darwin" ]]; then
    echo "--install-macos is only supported on macOS." >&2
    exit 2
  fi
  if ! command -v security >/dev/null 2>&1; then
    echo "Missing dependency: security (macOS)" >&2
    exit 1
  fi

  echo "Installing CA into macOS System Keychain (requires sudo)..." >&2
  sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$OUTPUT_PATH"
  echo "Installed." >&2
fi
