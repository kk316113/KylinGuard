#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
KYLINGUARD_HOME="${KYLINGUARD_HOME:-$REPO_ROOT}"
AGENT_DIR="$KYLINGUARD_HOME/agent-go"
AUDIT_CORE_URL="${AUDIT_CORE_URL:-http://127.0.0.1:8001}"
AGENT_GO_PORT="${AGENT_GO_PORT:-8080}"

if [[ ! -d "$AGENT_DIR" ]]; then
  printf 'agent-go directory not found: %s\n' "$AGENT_DIR" >&2
  exit 1
fi

check_audit_core() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsS "$AUDIT_CORE_URL/health" >/dev/null
    return $?
  fi
  if command -v python3 >/dev/null 2>&1; then
    python3 - "$AUDIT_CORE_URL/health" <<'PY'
import sys
import urllib.request

try:
    with urllib.request.urlopen(sys.argv[1], timeout=2) as response:
        raise SystemExit(0 if 200 <= response.status < 300 else 1)
except Exception:
    raise SystemExit(1)
PY
    return $?
  fi
  return 1
}

if check_audit_core; then
  printf 'audit-core-py is reachable at %s\n' "$AUDIT_CORE_URL"
else
  printf 'warning: audit-core-py is not reachable at %s; Go Agent will start with fallback audit behavior\n' "$AUDIT_CORE_URL" >&2
fi

cd "$AGENT_DIR"
export AUDIT_CORE_URL
export KYLIN_GUARD_AGENT_PORT="$AGENT_GO_PORT"

if [[ -x bin/kylin-guard-agent ]]; then
  printf 'starting built agent binary on port %s\n' "$AGENT_GO_PORT"
  exec ./bin/kylin-guard-agent
fi

if command -v go >/dev/null 2>&1; then
  printf 'starting go run ./cmd/server on port %s\n' "$AGENT_GO_PORT"
  exec go run ./cmd/server
fi

printf 'no built agent binary found and go is not available\n' >&2
exit 1
