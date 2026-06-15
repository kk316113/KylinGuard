#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
KYLINGUARD_HOME="${KYLINGUARD_HOME:-$REPO_ROOT}"
AUDIT_DIR="$KYLINGUARD_HOME/audit-core-py"

if ! command -v python3 >/dev/null 2>&1; then
  printf 'python3: not found; install Python 3 before installing audit-core-py\n' >&2
  exit 1
fi

if [[ ! -d "$AUDIT_DIR" ]]; then
  printf 'audit-core-py directory not found: %s\n' "$AUDIT_DIR" >&2
  exit 1
fi

cd "$AUDIT_DIR"

if grep -Eiq '(^|[-_[:alnum:]])(torch|transformers|faiss|sentence-transformers)([=<>[:space:]]|$)' requirements.txt; then
  printf 'refusing to install heavy model dependencies from requirements.txt\n' >&2
  exit 1
fi

python3 -m venv .venv
# shellcheck disable=SC1091
source .venv/bin/activate

printf 'installing audit-core-py requirements from %s/requirements.txt\n' "$AUDIT_DIR"
if ! python -m pip install -r requirements.txt; then
  printf 'pip install failed; check Python version, pip source, and network access\n' >&2
  exit 1
fi

printf 'audit-core-py dependencies installed in %s/.venv\n' "$AUDIT_DIR"
