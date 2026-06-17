#!/usr/bin/env bash
# Stage 15A: Check all demo services health.
# Detects deterministic vs mock LLM mode and checks appropriate expectations.

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

# --- Detect demo mode ---
DEMO_MODE="deterministic"
if [ -f "$APP_HOME/run/demo.env" ]; then
  # Parse env file safely (grep + cut, no source).
  local_enabled="$(grep '^export EINO_LLM_ENABLED=' "$APP_HOME/run/demo.env" 2>/dev/null | cut -d= -f2 || echo "")"
  local_provider="$(grep '^export EINO_LLM_PROVIDER=' "$APP_HOME/run/demo.env" 2>/dev/null | cut -d= -f2 || echo "")"
  if [ "$local_enabled" = "true" ]; then
    DEMO_MODE="mock"
  fi
fi
if [ "$DEMO_MODE" != "mock" ] && [ "${DEMO_MOCK_LLM:-false}" = "true" ]; then
  DEMO_MODE="mock"
fi
if [ "$DEMO_MODE" != "mock" ] && [ -f "$APP_HOME/run/mock-llm.pid" ]; then
  pid="$(cat "$APP_HOME/run/mock-llm.pid" 2>/dev/null || echo "")"
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
    DEMO_MODE="mock"
  fi
fi
if [ "$DEMO_MODE" = "deterministic" ] && [ -n "${OPENAI_COMPATIBLE_API_KEY:-}" ]; then
  DEMO_MODE="real-deepseek"
fi

printf '== KylinGuard Demo Health Check ==\n\n'
printf 'Mode: %s\n\n' "$DEMO_MODE"

# --- Basic service health ---
printf 'Services:\n'
check_http "audit-core-py" "http://127.0.0.1:${AUDIT_PORT}/health"
check_http "Go Agent" "http://127.0.0.1:${AGENT_PORT}/health"
check_http "Frontend" "http://127.0.0.1:${FRONTEND_PORT}"

# --- Check Go Agent LLM mode via API ---
printf '\nGo Agent LLM mode:\n'
ENDPOINT="http://127.0.0.1:${AGENT_PORT}/api/agent/run-eino"
PAYLOAD='{"task":"check SSH login anomaly"}'
RESP=$(curl -sf -X POST "$ENDPOINT" -H "Content-Type: application/json" -d "$PAYLOAD" 2>/dev/null || echo "")

if [ -z "$RESP" ]; then
  printf '  %-30s %s\n' "API check" "[FAIL] (no response from run-eino)"
  FAIL=$((FAIL + 1))
else
  # Use python3 for multi-path JSON extraction.
  python3 - "$RESP" "$DEMO_MODE" <<'PYCHECK'
import json, sys

resp = json.loads(sys.argv[1])
mode = sys.argv[2]
errors = []

def get_value(*paths):
    """Try multiple dotted paths, return first non-None value."""
    for path in paths:
        parts = path.split(".")
        val = resp
        try:
            for p in parts:
                if p.isdigit():
                    val = val[int(p)]
                elif isinstance(val, dict):
                    val = val.get(p)
                else:
                    val = None
                    break
            if val is not None:
                return val
        except (KeyError, IndexError, TypeError, ValueError):
            continue
    return None

def get_bool(*paths):
    """Try multiple paths, return boolean."""
    v = get_value(*paths)
    if v is True or v == "True" or v == "true":
        return True
    if v is False or v == "False" or v == "false":
        return False
    return None

# Read from security_report.audit_metadata, security_report top-level, and reasoning_trace spans.
llm_enabled = get_bool(
    "security_report.llm_enabled",
    "security_report.audit_metadata.llm_enabled",
)
chat_model = get_value(
    "security_report.chat_model",
    "security_report.audit_metadata.chat_model",
)
remote_llm_used = get_bool(
    "security_report.remote_llm_used",
    "security_report.audit_metadata.remote_llm_used",
)
plan_scenario = get_value("plan.scenario", "")

# Fallback: check reasoning_trace chat_model span attributes.
if llm_enabled is None or chat_model is None or remote_llm_used is None:
    for span in (resp.get("reasoning_trace") or {}).get("spans") or []:
        if span.get("type") == "chat_model":
            attrs = span.get("attributes") or {}
            if llm_enabled is None:
                llm_enabled = attrs.get("llm_enabled")
            if chat_model is None:
                chat_model = attrs.get("provider")
            if remote_llm_used is None:
                remote_llm_used = attrs.get("remote_llm_used")
            break

# Coerce booleans to string for display.
def b(v):
    if v is True:
        return "True"
    if v is False:
        return "False"
    return str(v) if v is not None else "None"

if mode == "mock" or mode == "real-deepseek":
    ok = True
    # Extract agent loop fields.
    agent_mode = get_value("agent_mode", "")
    agent_steps = resp.get("agent_steps") or []
    tool_trace = resp.get("tool_trace") or []
    final_answer = get_value("final_answer", "")
    audit_result = resp.get("audit_result") or {}

    if llm_enabled is not True:
        print(f'  %-30s %s' % ("llm_enabled", f"[FAIL] (expected True, got {b(llm_enabled)})"))
        ok = False
    if chat_model == "deterministic-stub" or not chat_model:
        print(f'  %-30s %s' % ("chat_model", f"[FAIL] (expected remote LLM, got {chat_model})"))
        ok = False
    if remote_llm_used is not True and mode == "mock":
        print(f'  %-30s %s' % ("remote_llm_used", f"[FAIL] (expected True, got {b(remote_llm_used)})"))
        ok = False
    if agent_mode != "agent_loop":
        print(f'  %-30s %s' % ("agent_mode", f"[FAIL] (expected agent_loop, got {agent_mode})"))
        ok = False
    if not final_answer:
        print(f'  %-30s %s' % ("final_answer", "[FAIL] (expected non-empty)"))
        ok = False
    if mode == "mock":
        if len(agent_steps) < 3:
            print(f'  %-30s %s' % ("agent_steps", f"[FAIL] (expected >=3, got {len(agent_steps)})"))
            ok = False
        if len(tool_trace) < 3:
            print(f'  %-30s %s' % ("tool_trace", f"[FAIL] (expected >=3, got {len(tool_trace)})"))
            ok = False
        # Check that SSH steps include expected tools.
        step_tools = [s.get("tool_name") for s in agent_steps]
        for required in ["service_status", "port_checker"]:
            if required not in step_tools:
                print(f'  %-30s %s' % ("step_tools", f"[FAIL] (missing {required} in {step_tools})"))
                ok = False
    if not audit_result.get("decision"):
        print(f'  %-30s %s' % ("audit_result", "[FAIL] (missing decision)"))
        ok = False
    if ok:
        label = "mock-openai-compatible" if mode == "mock" else "real-deepseek"
        print(f'  %-30s %s' % ("mode", f"[OK] {label}"))
        print(f'  %-30s %s' % ("llm_enabled", "[OK] True"))
        print(f'  %-30s %s' % ("chat_model", f"[OK] {chat_model}"))
        if remote_llm_used is True:
            print(f'  %-30s %s' % ("remote_llm_used", "[OK] True"))
        if mode == "mock":
            print(f'  %-30s %s' % ("agent_steps", f"[OK] {len(agent_steps)} steps"))
            print(f'  %-30s %s' % ("tool_trace", f"[OK] {len(tool_trace)} tools"))
else:
    ok = True
    if llm_enabled is not False:
        print(f'  %-30s %s' % ("llm_enabled", f"[FAIL] (expected False, got {b(llm_enabled)})"))
        ok = False
    if chat_model != "deterministic-stub":
        print(f'  %-30s %s' % ("chat_model", f"[FAIL] (expected deterministic-stub, got {chat_model})"))
        ok = False
    if ok:
        print(f'  %-30s %s' % ("mode", "[OK] deterministic"))
        print(f'  %-30s %s' % ("llm_enabled", "[OK] False"))
        print(f'  %-30s %s' % ("chat_model", "[OK] deterministic-stub"))

if not errors:
    pass
print("")  # newline separator

sys.exit(0 if not errors else 1)
PYCHECK
  PYEXIT=$?
  if [ "$PYEXIT" -ne 0 ]; then
    FAIL=$((FAIL + 1))
  else
    PASS=$((PASS + 1))
  fi
fi

# --- Mock LLM server check ---
printf 'Mock LLM:\n'
if [ "$DEMO_MODE" = "mock" ]; then
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

# --- PID files ---
printf '\nPid files:\n'
check_pid "agent-go pid" "$APP_HOME/run/agent-go.pid"
check_pid "audit-core pid" "$APP_HOME/run/audit-core.pid"
if [ -f "$APP_HOME/run/frontend.pid" ]; then
  check_pid "frontend pid" "$APP_HOME/run/frontend.pid"
else
  printf '  %-30s %s\n' "frontend pid" "[NO PID FILE]"
fi

# --- Summary ---
printf '\n'
printf '  Results: %d passed, %d failed, %d skipped\n' "$PASS" "$FAIL" "$SKIP"
printf '\n'
printf '  Demo URL: http://127.0.0.1:%s\n' "$FRONTEND_PORT"

if [ "$FAIL" -gt 0 ]; then
  printf '\n  WARNING: Some checks failed. Check logs/ for details.\n'
  exit 1
fi
printf '  All services healthy.\n'
