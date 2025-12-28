#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT_DIR/deploy/bootstrap-user.sh"

# Minimal checks for argument validation and SSH invocation.
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
cat >"$tmp_dir/bin/ssh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

for arg in "$@"; do
  if [[ "$arg" == *'printf %s "$HOME"'* ]]; then
    printf "%s" "/home/test"
    exit 0
  fi
done

if [[ -z "${SSH_LOG:-}" ]]; then
  exit 0
fi

printf '%s\n' "$@" >>"$SSH_LOG"

if [[ -n "${SSH_MOCK_OUTPUT:-}" ]]; then
  if [[ "${SSH_MOCK_OUTPUT_STREAM:-stderr}" == "stdout" ]]; then
    printf '%s\n' "$SSH_MOCK_OUTPUT"
  else
    printf '%s\n' "$SSH_MOCK_OUTPUT" >&2
  fi
fi

if [[ -n "${SSH_MOCK_STATUS:-}" ]]; then
  exit "$SSH_MOCK_STATUS"
fi

exit 0
EOF
chmod +x "$tmp_dir/bin/ssh"

export PATH="$tmp_dir/bin:$PATH"
export SSH_LOG="$tmp_dir/ssh.log"

set +e
"$SCRIPT" >/dev/null 2>&1
status=$?
set -e
if [[ "$status" -ne 2 ]]; then
  fail "expected missing args to exit 2, got $status"
fi

if ! "$SCRIPT" --username alice --password secret --host example --remote-dir ~/apps/cooking_app >/dev/null 2>&1; then
  fail "expected script to succeed"
fi

if [[ ! -s "$SSH_LOG" ]]; then
  fail "expected ssh to be invoked"
fi

if ! grep -q "USERNAME_B64=" "$SSH_LOG"; then
  fail "expected USERNAME_B64 in ssh command"
fi
if ! grep -q "PASSWORD_B64=" "$SSH_LOG"; then
  fail "expected PASSWORD_B64 in ssh command"
fi

export SSH_MOCK_OUTPUT="bootstrap refused: users table is not empty"
export SSH_MOCK_STATUS=1
if ! "$SCRIPT" --username alice --password secret --host example --remote-dir ~/apps/cooking_app >/dev/null 2>&1; then
  fail "expected existing-user error to be ignored"
fi
unset SSH_MOCK_OUTPUT SSH_MOCK_STATUS

echo "ok"
