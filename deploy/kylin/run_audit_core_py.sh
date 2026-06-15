#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
AUDIT_DIR="$ROOT_DIR/audit-core-py"

cd "$AUDIT_DIR"

if [ -f .venv/bin/activate ]; then
  . .venv/bin/activate
fi

PYTHON_BIN=${PYTHON_BIN:-python3}
AUDIT_CORE_HOST=${AUDIT_CORE_HOST:-0.0.0.0}
AUDIT_CORE_PORT=${AUDIT_CORE_PORT:-8090}

exec "$PYTHON_BIN" -m uvicorn app.main:app --host "$AUDIT_CORE_HOST" --port "$AUDIT_CORE_PORT"
