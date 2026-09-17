#!/usr/bin/env bash
# agyp-sync: Synchronizes skills, hooks, tools, and shared configurations across all agyp profiles.
set -euo pipefail

if command -v agyp >/dev/null 2>&1; then
  exec agyp sync "$@"
fi

BASE_HOME="${HOME:-$(eval echo ~${USER:-})}"
if [ -x "$BASE_HOME/.local/bin/agyp" ]; then
  exec "$BASE_HOME/.local/bin/agyp" sync "$@"
fi

echo "agyp command not found in PATH or ~/.local/bin" >&2
exit 1
