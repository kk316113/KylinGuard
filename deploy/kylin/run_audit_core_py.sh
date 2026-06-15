#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
KYLINGUARD_HOME="${KYLINGUARD_HOME:-$REPO_ROOT}"
AUDIT_DIR="$KYLINGUARD_HOME/audit-core-py"
TRACESHIELD_CORE_PATH="${TRACESHIELD_CORE_PATH:-/opt/traceshield-core}"
AUDIT_CORE_HOST="${AUDIT_CORE_HOST:-0.0.0.0}"
AUDIT_CORE_PORT="${AUDIT_CORE_PORT:-8001}"

if [[ ! -d "$AUDIT_DIR" ]]; then
  printf 'audit-core-py directory not found: %s\n' "$AUDIT_DIR" >&2
  exit 1
fi

if [[ ! -d "$AUDIT_DIR/.venv" ]]; then
  printf 'audit-core-py virtualenv not found: %s/.venv\n' "$AUDIT_DIR" >&2
  printf 'run deploy/kylin/install_audit_core_py.sh first\n' >&2
  exit 1
fi

if [[ ! -d "$TRACESHIELD_CORE_PATH" ]]; then
  printf 'TRACESHIELD_CORE_PATH does not exist: %s\n' "$TRACESHIELD_CORE_PATH" >&2
  exit 1
fi

cd "$AUDIT_DIR"
# shellcheck disable=SC1091
source .venv/bin/activate

export TRACESHIELD_CORE_PATH
printf 'starting audit-core-py on %s:%s with TRACESHIELD_CORE_PATH=%s\n' "$AUDIT_CORE_HOST" "$AUDIT_CORE_PORT" "$TRACESHIELD_CORE_PATH"
exec python -m uvicorn app.main:app --host "$AUDIT_CORE_HOST" --port "$AUDIT_CORE_PORT"
