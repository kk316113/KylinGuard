#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
AUDIT_DIR="$ROOT_DIR/audit-core-py"

if ! command -v python3 >/dev/null 2>&1; then
  printf 'python3: not found\n' >&2
  exit 1
fi

cd "$AUDIT_DIR"
python3 -m venv .venv
. .venv/bin/activate
python -m pip install --upgrade pip
python -m pip install -r requirements.txt

printf 'audit-core-py dependencies installed\n'
