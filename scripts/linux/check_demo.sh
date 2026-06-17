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
    printf '  %-30s %s\n' "$name" "[OK]"
    PASS=$((PASS + 1))
  else
    printf '  %-30s %s\n' "$name" "[FAIL]"
    FAIL=$((FAIL + 1))
  fi
}

check_json_value() {
  # POST to agent-run-eino and check a JSON path matches expected value.
  local name="$1" url="$2" payload="$3" jq_path="$4" expected="$5"
  local resp
  resp=$(curl -sf -X POST "$url" -H "Content-Type: application/json" -d "$payload" 2>/dev/null || echo "")
  if [ -z "$resp" ]; then
    printf '  %-30s %s\n' "$name" "[FAIL] (no response)"
    FAIL=$((FAIL + 1))
    return
  fi
  local actual
  actual=$(echo "$resp" | python3 -c "
import json, sys
try:
    body = json.loads(sys.stdin.read())
    val = body
    for p in '$jq_path'.split('.'):
        if p.isdigit():
            val = val[int(p)]
        else:
            val = val.get(p, None) if isinstance(val, dict) else None
        if val is None:
            break
    print(val if val is not None else '')
except Exception:
    print('')
" 2>/dev/null || echo "")
  if [ "$actual" = "$expected" ]; then
    printf '  %-30s %s\n' "$name" "[OK] ($expected)"
    PASS=$((PASS + 1))
  else
    printf '  %-30s %s\n' "$name" "[FAIL] (expected $expected, got $actual)"
    FAIL=$((FAIL + 1))
  fi
}

printf '== KylinGuard Demo Health Check ==\n\n'

printf 'Services:\n'
check_http "audit-core-py" "http://127.0.0.1:${AUDIT_PORT}/health"
check_http "Go Agent" "http://127.0.0.1:${AGENT_PORT}/health"
check_http "Frontend" "http://127.0.0.1:${FRONTEND_PORT}"

# Check Go Agent LLM mode.
printf '\nGo Agent LLM mode:\n'
check_json_value "llm_enabled" \
  "http://127.0.0.1:${AGENT_PORT}/api/agent/run-eino" \
  '{"task":"check system resource usage"}' \
  "security_report.audit_metadata.llm_enabled" \
  "True"
check_json_value "chat_model" \
  "http://127.0.0.1:${AGENT_PORT}/api/agent/run-eino" \
  '{"task":"check system resource usage"}' \
  "security_report.audit_metadata.chat_model" \
  "deterministic-stub"

# Check mock LLM if pid file exists or DEMO_MOCK_LLM is true.
printf '\nMock LLM:\n'
if [ -f "$APP_HOME/run/mock-llm.pid" ] || [ "${DEMO_MOCK_LLM:-false}" = "true" ]; then
  if curl -sf -X POST "http://127.0.0.1:${MOCK_LLM_PORT}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{"messages":[{"role":"user","content":"ping"}]}' >/dev/null 2>&1; then
    printf '  %-30s %s\n' "Mock LLM server" "[OK]"
    PASS=$((PASS + 1))
  else
    printf '  %-30s %s\n' "Mock LLM server" "[FAIL]"
    FAIL=$((FAIL + 1))
  fi
else
  printf '  %-30s %s\n' "Mock LLM server" "[SKIPPED]"
  SKIP=$((SKIP + 1))
fi

printf '\nPid files:\n'
check_pid() {
  local name="$1" pid_file="$2"
  if [ -f "$pid_file" ] && kill -0 "$(cat "$pid_file" 2>/dev/null)" 2>/dev/null; then
    printf '  %-30s %s\n' "$name" "[OK]"
    PASS=$((PASS + 1))
  else
    printf '  %-30s %s\n' "$name" "[FAIL]"
    FAIL=$((FAIL + 1))
  fi
}
check_pid "agent-go pid" "$APP_HOME/run/agent-go.pid"
check_pid "audit-core pid" "$APP_HOME/run/audit-core.pid"

if [ -f "$APP_HOME/run/frontend.pid" ]; then
  check_pid "frontend pid" "$APP_HOME/run/frontend.pid"
else
  printf '  %-30s %s\n' "frontend pid" "[NO PID FILE]"
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
