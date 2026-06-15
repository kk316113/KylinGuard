#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
AGENT_DIR="$ROOT_DIR/agent-go"

if ! command -v go >/dev/null 2>&1; then
  printf 'go: not found\n' >&2
  exit 1
fi

cd "$AGENT_DIR"
mkdir -p bin
go mod tidy
go build -o bin/kylin-guard-agent ./cmd/server

printf 'built %s/bin/kylin-guard-agent\n' "$AGENT_DIR"
