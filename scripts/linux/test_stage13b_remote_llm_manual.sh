#!/usr/bin/env bash
# Stage 13B: Manual Remote LLM Verification Script
# This script verifies that run-eino works with a real or mock LLM endpoint.
# It is NOT part of the default CI/VM E2E; it requires manual setup.
#
# Usage:
#   export EINO_LLM_ENABLED=true
#   export EINO_LLM_PROVIDER=openai_compatible
#   export EINO_LLM_ENDPOINT=http://127.0.0.1:8800/v1/chat/completions
#   export EINO_LLM_MODEL=mock-model
#   export EINO_LLM_API_KEY=sk-mock-key
#   bash test_stage13b_remote_llm_manual.sh
#
# Or with mock server:
#   python3 ../dev/mock_openai_compatible_server.py &
#   bash test_stage13b_remote_llm_manual.sh
#   kill %1

set -euo pipefail

KYLIN_GUARD_AGENT_URL="${KYLIN_GUARD_AGENT_URL:-http://127.0.0.1:8080}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'required command not found: %s\n' "$1" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd python3

printf '\n== Stage 13B Remote LLM Manual Verification ==\n\n'

# Check required environment variables.
if [ "${EINO_LLM_ENABLED:-false}" != "true" ]; then
  printf 'ERROR: EINO_LLM_ENABLED must be set to "true".\n'
  printf '  export EINO_LLM_ENABLED=true\n'
  exit 1
fi

if [ -z "${EINO_LLM_API_KEY:-}" ]; then
  printf 'WARNING: EINO_LLM_API_KEY is not set.\n'
  printf '  If using the mock server, set it to any value, e.g.:\n'
  printf '  export EINO_LLM_API_KEY=sk-mock-key\n'
fi

if [ -z "${EINO_LLM_ENDPOINT:-}" ]; then
  printf 'WARNING: EINO_LLM_ENDPOINT is not set.\n'
fi

printf 'Configuration (API_KEY redacted):\n'
printf '  EINO_LLM_ENABLED=%s\n' "${EINO_LLM_ENABLED:-<unset>}"
printf '  EINO_LLM_PROVIDER=%s\n' "${EINO_LLM_PROVIDER:-<unset>}"
printf '  EINO_LLM_ENDPOINT=%s\n' "${EINO_LLM_ENDPOINT:-<unset>}"
printf '  EINO_LLM_MODEL=%s\n' "${EINO_LLM_MODEL:-<unset>}"
printf '  API_KEY: [REDACTED]\n'
printf '\n'

# Test 1: Check Go Agent health.
printf '1. Checking Go Agent health...\n'
HEALTH=$(curl -s -f "$KYLIN_GUARD_AGENT_URL/health" 2>&1 || true)
if [ -z "$HEALTH" ]; then
  printf '  FAIL: Go Agent is not running at %s\n' "$KYLIN_GUARD_AGENT_URL"
  printf '  Start the agent with: SKIP_E2E=true bash start_all.sh\n'
  exit 1
fi
printf '  PASS: Go Agent is running.\n\n'

# Test 2: Call run-eino with SSH anomaly task.
printf '2. Calling run-eino with: check SSH login anomaly\n'
RESPONSE_FILE="/tmp/kylin_guard_stage13b_response.json"
curl -s -f -X POST "$KYLIN_GUARD_AGENT_URL/api/agent/run-eino" \
  -H "Content-Type: application/json" \
  -d '{"task":"check SSH login anomaly"}' > "$RESPONSE_FILE" 2>&1 || {
  rc=$?
  printf '  FAIL: curl returned exit code %d\n' "$rc"
  if [ -s "$RESPONSE_FILE" ]; then
    printf '  Response (first 500 chars):\n'
    head -c 500 "$RESPONSE_FILE" | python3 -c "
import sys; data = sys.stdin.buffer.read()
try:
    decoded = data.decode('utf-8')
except:
    decoded = repr(data[:500])
# Redact any API key patterns
import re
decoded = re.sub(r'sk-[A-Za-z0-9]{10,}', '[REDACTED]', decoded)
print(decoded[:500])
"
  fi
  exit 1
}

RESPONSE=$(cat "$RESPONSE_FILE")
if [ -z "$RESPONSE" ]; then
  printf '  FAIL: Empty response from run-eino\n'
  exit 1
fi

# Use python3 to validate the response with detailed error reporting.
printf '  Response saved to: %s\n' "$RESPONSE_FILE"
python3 - "$RESPONSE" <<'PYCHECK'
import json, sys

RESPONSE_FILE = "/tmp/stage13b_remote_llm_response.json"

resp = json.loads(sys.argv[1])
errors = []

def check(cond, msg):
    if not cond:
        errors.append(msg)

def get_first(*paths):
    """Try multiple paths to find a value, return the first non-None."""
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

def print_summary():
    print("\n  Response summary:")
    print(f"    task: {resp.get('task', '?')}")
    print(f"    decision: {resp.get('decision', '?')}")
    print(f"    summary: {resp.get('summary', '?')}")
    print(f"    audit_result.method: {(resp.get('audit_result') or {}).get('method', '?')}")
    print(f"    tool_trace length: {len(resp.get('tool_trace') or [])}")
    print(f"    plan.scenario: {(resp.get('plan') or {}).get('scenario', '?')}")
    print(f"    plan.steps: {[(s.get('tool_name','?')) for s in (resp.get('plan') or {}).get('steps') or []]}")
    rt = resp.get('reasoning_trace') or {}
    print(f"    reasoning_trace: {rt.get('trace_id', '?')} spans={len(rt.get('spans') or [])}")
    report = resp.get('security_report') or {}
    meta = report.get('audit_metadata') or {}
    print(f"    security_report.title: {report.get('title', '?')}")
    print(f"    security_report.route: {meta.get('route', '?')}")
    print(f"    security_report.runtime: {meta.get('runtime', '?')}")
    print(f"    security_report.llm_enabled: {meta.get('llm_enabled', '?')}")
    print(f"    security_report.chat_model: {meta.get('chat_model', '?')}")
    print(f"    security_report.chat_model_adapter: {meta.get('chat_model_adapter', '?')}")
    print(f"    security_report.remote_llm_used: {meta.get('remote_llm_used', '?')}")
    print(f"    security_report.fallback_used: {meta.get('fallback_used', '?')}")
    if meta.get("fallback_reason"):
        print(f"    security_report.fallback_reason: {meta.get('fallback_reason')}")
    print(f"    security_report.eino_runtime_version: {meta.get('eino_runtime_version', '?')}")
    # Chat model span attributes
    for span in rt.get('spans') or []:
        if span.get('type') == 'chat_model':
            print(f"    chat_model span attributes: {json.dumps(span.get('attributes', {}))}")
            break

# Top-level structure
check("task" in resp, "response missing 'task'")
check("decision" in resp, "response missing 'decision'")
check(resp.get("decision") != "", "decision is empty")

# Security report
report = resp.get("security_report") or {}
meta = report.get("audit_metadata") or {}

if not report or not meta:
    print("\n  WARNING: security_report or audit_metadata is missing/empty.")
    print_summary()
    check(False, "security_report or audit_metadata missing")
else:
    check(meta.get("route") == "eino-runtime", f"expected route=eino-runtime, got {meta.get('route')}")
    check(meta.get("runtime") == "eino", f"expected runtime=eino, got {meta.get('runtime')}")
    check(meta.get("llm_enabled") is True, f"expected llm_enabled=true, got {meta.get('llm_enabled')}")
    check(meta.get("chat_model_adapter") == "interface-v1", f"expected chat_model_adapter=interface-v1, got {meta.get('chat_model_adapter')}")
    check(meta.get("eino_runtime_version") == "stage13a-v1", f"expected eino_runtime_version=stage13a-v1, got {meta.get('eino_runtime_version')}")

    # Read remote_llm_used / fallback_used from multiple locations (priority order).
    remote = get_first(
        "security_report.remote_llm_used",
        "security_report.audit_metadata.remote_llm_used",
        "reasoning_trace.spans.attributes.remote_llm_used",
    )
    fallback = get_first(
        "security_report.fallback_used",
        "security_report.audit_metadata.fallback_used",
        "reasoning_trace.spans.attributes.fallback_used",
    )
    # If not found at top level, try audit_metadata.
    if remote is None:
        remote = meta.get("remote_llm_used")
    if fallback is None:
        fallback = meta.get("fallback_used")
    # Last resort: walk chat_model span attributes.
    if remote is None or fallback is None:
        for span in (resp.get("reasoning_trace") or {}).get("spans") or []:
            if span.get("type") == "chat_model":
                attrs = span.get("attributes") or {}
                if remote is None:
                    remote = attrs.get("remote_llm_used")
                if fallback is None:
                    fallback = attrs.get("fallback_used")
                break

    check(remote is True or fallback is True,
          f"expected remote_llm_used=true or fallback_used=true, got remote={remote} fallback={fallback}")
    if fallback is True:
        fb_reason = meta.get("fallback_reason", "")
        if not fb_reason:
            # Try chat_model span.
            for span in (resp.get("reasoning_trace") or {}).get("spans") or []:
                if span.get("type") == "chat_model":
                    fb_reason = (span.get("attributes") or {}).get("fallback_reason", "")
                    break
        check(fb_reason != "", "fallback_used=true but fallback_reason is empty")
        for pat in ["sk-", "Bearer", "Authorization"]:
            if pat in fb_reason:
                check(False, f"fallback_reason contains sensitive pattern '{pat}'")

    # Check remote_llm_used specifics — plan must match SSH task.
    plan = resp.get("plan") or {}
    scenario = plan.get("scenario", "")
    check(scenario == "ssh_anomaly_check",
          f"expected scenario=ssh_anomaly_check for SSH task, got {scenario}")
    if remote is True:
        traces = resp.get("tool_trace") or []
        check(len(traces) > 0, f"expected tool_trace for remote LLM plan, got empty")

# Reasoning trace
rt = resp.get("reasoning_trace") or {}
spans = rt.get("spans") or []
check(len(spans) > 0, "reasoning_trace.spans is empty")

# Check for sensitive data in reasoning trace
for span in spans:
    attrs = span.get("attributes") or {}
    for k, v in attrs.items():
        kl = k.lower()
        for pat in ["api_key", "authorization", "bearer", "secret", "password"]:
            if pat in kl:
                check(False, f"sensitive key '{k}' found in reasoning_trace span '{span.get('type')}'")
        if isinstance(v, str):
            vl = v.lower()
            if "bearer " in vl or "-----begin" in vl:
                check(False, f"sensitive value in reasoning_trace span '{span.get('type')}' attr '{k}'")

# Check that API key is not leaked in response body
resp_str = json.dumps(resp)
# Only flag real-looking keys (not "sk-mock-key" or "sk-test")
if "sk-" in resp_str and "sk-mock" not in resp_str and "sk-test" not in resp_str:
    check(False, "possible API key leak in response body (sk- pattern found)")

if errors:
    print("\nFAILURES:")
    for e in errors:
        print(f"  - {e}")
    print_summary()
    raise SystemExit(1)
else:
    print("\n  PASS: All checks passed.")
    print(f"  decision={resp.get('decision','?')}")
    print(f"  plan.scenario={(resp.get('plan') or {}).get('scenario','?')}")
    print(f"  llm_enabled={meta.get('llm_enabled','?')}")
    print(f"  remote_llm_used={meta.get('remote_llm_used','?')}")
    print(f"  fallback_used={meta.get('fallback_used','?')}")
    if meta.get("fallback_reason"):
        print(f"  fallback_reason={meta.get('fallback_reason')}")
    span_types = [s["type"] for s in spans]
    print(f"  span_types={span_types}")

print("")
print("Stage 13B remote LLM manual check passed.")
PYCHECK
