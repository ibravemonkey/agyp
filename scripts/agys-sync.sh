#!/usr/bin/env bash
# agys-sync: Synchronizes skills, hooks, tools, and shared configurations across all agys profiles.
set -euo pipefail

if command -v agys >/dev/null 2>&1; then
  exec agys sync "$@"
fi

BASE_HOME="${HOME:-$(eval echo ~${USER:-})}"
if [ -x "$BASE_HOME/.local/bin/agys" ]; then
  exec "$BASE_HOME/.local/bin/agys" sync "$@"
fi

echo "agys command not found in PATH or ~/.local/bin" >&2
exit 1
