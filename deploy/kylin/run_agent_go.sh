#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
AGENT_DIR="$ROOT_DIR/agent-go"

cd "$AGENT_DIR"

if [ -x bin/kylin-guard-agent ]; then
  exec ./bin/kylin-guard-agent
fi

if command -v go >/dev/null 2>&1; then
  exec go run ./cmd/server
fi

printf 'no built agent binary found and go is not available\n' >&2
exit 1
