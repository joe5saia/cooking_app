#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT_DIR/deploy/deploy.sh"

# Minimal checks for deploy defaults with stubbed external commands.
fail() {
  echo "FAIL: $1" >&2
  exit 1
}

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

mkdir -p "$tmp_dir/bin"
cat >"$tmp_dir/bin/ssh" <<'SSH_EOF'
#!/usr/bin/env bash
set -euo pipefail

for arg in "$@"; do
  if [[ "$arg" == *'printf %s "$HOME"'* ]]; then
    printf "%s" "/home/test"
    exit 0
  fi
done

if [[ -n "${SSH_LOG:-}" ]]; then
  printf '%s\n' "$@" >>"$SSH_LOG"
fi

exit 0
SSH_EOF
chmod +x "$tmp_dir/bin/ssh"

cat >"$tmp_dir/bin/rsync" <<'RSYNC_EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${RSYNC_LOG:-}" ]]; then
  printf '%s\n' "$@" >>"$RSYNC_LOG"
fi

exit 0
RSYNC_EOF
chmod +x "$tmp_dir/bin/rsync"

cat >"$tmp_dir/bin/make" <<'MAKE_EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${MAKE_LOG:-}" ]]; then
  printf '%s\n' "$@" >>"$MAKE_LOG"
fi

exit 0
MAKE_EOF
chmod +x "$tmp_dir/bin/make"

cat >"$tmp_dir/bin/curl" <<'CURL_EOF'
#!/usr/bin/env bash
set -euo pipefail

echo "ok"
exit 0
CURL_EOF
chmod +x "$tmp_dir/bin/curl"

export PATH="$tmp_dir/bin:$PATH"
export SSH_LOG="$tmp_dir/ssh.log"
export RSYNC_LOG="$tmp_dir/rsync.log"
export MAKE_LOG="$tmp_dir/make.log"
export POSTGRES_PASSWORD="testpass"

run_deploy() {
  : >"$SSH_LOG"
  : >"$RSYNC_LOG"
  : >"$MAKE_LOG"
  "$SCRIPT" "$@" >/dev/null 2>&1
}

assert_log_contains() {
  local log_file="$1"
  local pattern="$2"
  local message="$3"

  if ! grep -Fq -- "$pattern" "$log_file"; then
    fail "$message"
  fi
}

assert_log_not_contains() {
  local log_file="$1"
  local pattern="$2"
  local message="$3"

  if grep -Fq -- "$pattern" "$log_file"; then
    fail "$message"
  fi
}

run_deploy
assert_log_contains "$SSH_LOG" "-f deploy/compose.lan.yaml" "expected LAN HTTPS compose file by default"
assert_log_contains "$SSH_LOG" "SESSION_COOKIE_SECURE=true" "expected SESSION_COOKIE_SECURE=true by default"

run_deploy --no-lan-https
assert_log_contains "$SSH_LOG" "-f deploy/compose.yaml" "expected standard compose file with --no-lan-https"
assert_log_contains "$SSH_LOG" "SESSION_COOKIE_SECURE=false" "expected SESSION_COOKIE_SECURE=false with --no-lan-https"
assert_log_not_contains "$SSH_LOG" "-f deploy/compose.lan.yaml" "did not expect LAN HTTPS compose file with --no-lan-https"

echo "ok"
