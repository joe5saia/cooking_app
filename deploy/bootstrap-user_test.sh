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

echo "ok"
