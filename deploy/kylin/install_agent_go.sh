#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
KYLINGUARD_HOME="${KYLINGUARD_HOME:-$REPO_ROOT}"
AGENT_DIR="$KYLINGUARD_HOME/agent-go"

if ! command -v go >/dev/null 2>&1; then
  printf 'go: not found; install Go before building agent-go\n' >&2
  exit 1
fi

if [[ ! -d "$AGENT_DIR" ]]; then
  printf 'agent-go directory not found: %s\n' "$AGENT_DIR" >&2
  exit 1
fi

cd "$AGENT_DIR"
mkdir -p bin

printf 'running go mod tidy in %s\n' "$AGENT_DIR"
go mod tidy

printf 'running go test ./...\n'
go test ./...

printf 'building bin/kylin-guard-agent\n'
go build -o bin/kylin-guard-agent ./cmd/server

printf 'built %s/bin/kylin-guard-agent\n' "$AGENT_DIR"
