#!/usr/bin/env bash
# bash -n + shellcheck over install.sh, scripts/*.sh and every *.sh under tests/install/.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if ! command -v shellcheck >/dev/null 2>&1; then
  echo "lint: installing shellcheck via Homebrew"
  brew install shellcheck
fi

files=()
for f in install.sh scripts/*.sh $(find tests/install -name '*.sh' -not -path '*/artifacts/*' | sort); do
  [ -f "$f" ] && files+=("$f")
done
[ "${#files[@]}" -gt 0 ] || { echo "lint: no shell files found" >&2; exit 1; }

rc=0
for f in "${files[@]}"; do
  if bash -n "$f" && shellcheck -x "$f"; then echo "ok   $f"; else echo "FAIL $f"; rc=1; fi
done
exit "$rc"
