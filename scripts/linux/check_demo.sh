#!/usr/bin/env bash
# Stage 15A: Check all demo services health.

set -euo pipefail

APP_HOME="${KYLINGUARD_HOME:-/opt/kylin-guard-agent}"
AUDIT_PORT="${AUDIT_CORE_PORT:-8001}"
AGENT_PORT="${AGENT_GO_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"
MOCK_LLM_PORT="${MOCK_LLM_PORT:-8800}"
PASS=0
FAIL=0
SKIP=0

check_http() {
  local name="$1" url="$2"
  if curl -sf "$url" >/dev/null 2>&1; then
    printf '  %-20s %s\n' "$name" "[OK]"
    PASS=$((PASS + 1))
  else
    printf '  %-20s %s\n' "$name" "[FAIL]"
    FAIL=$((FAIL + 1))
  fi
}

check_pid() {
  local name="$1" pid_file="$2"
  if [ -f "$pid_file" ] && kill -0 "$(cat "$pid_file" 2>/dev/null)" 2>/dev/null; then
    printf '  %-20s %s\n' "$name" "[OK]"
    PASS=$((PASS + 1))
  else
    printf '  %-20s %s\n' "$name" "[FAIL]"
    FAIL=$((FAIL + 1))
  fi
}

printf '== KylinGuard Demo Health Check ==\n\n'

printf 'Services:\n'
check_http "audit-core-py" "http://127.0.0.1:${AUDIT_PORT}/health"
check_http "Go Agent" "http://127.0.0.1:${AGENT_PORT}/health"
check_http "Frontend" "http://127.0.0.1:${FRONTEND_PORT}"

# Check mock LLM if pid file exists or DEMO_MOCK_LLM is true
if [ -f "$APP_HOME/run/mock-llm.pid" ] || [ "${DEMO_MOCK_LLM:-false}" = "true" ]; then
  if curl -sf -X POST "http://127.0.0.1:${MOCK_LLM_PORT}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{"messages":[{"role":"user","content":"ping"}]}' >/dev/null 2>&1; then
    printf '  %-20s %s\n' "Mock LLM" "[OK]"
    PASS=$((PASS + 1))
  else
    printf '  %-20s %s\n' "Mock LLM" "[FAIL]"
    FAIL=$((FAIL + 1))
  fi
else
  printf '  %-20s %s\n' "Mock LLM" "[SKIPPED]"
  SKIP=$((SKIP + 1))
fi

printf '\nPid files:\n'
check_pid "agent-go pid" "$APP_HOME/run/agent-go.pid"
check_pid "audit-core pid" "$APP_HOME/run/audit-core.pid"

if [ -f "$APP_HOME/run/frontend.pid" ]; then
  check_pid "frontend pid" "$APP_HOME/run/frontend.pid"
else
  printf '  %-20s %s\n' "frontend pid" "[NO PID FILE]"
fi

printf '\n'
printf '  Results: %d passed, %d failed, %d skipped\n' "$PASS" "$FAIL" "$SKIP"
printf '\n'
printf '  Demo URL: http://127.0.0.1:%s\n' "$FRONTEND_PORT"

if [ "$FAIL" -gt 0 ]; then
  printf '\n  WARNING: Some services are not healthy. Check logs/ for details.\n'
  exit 1
fi
printf '  All services healthy.\n'
