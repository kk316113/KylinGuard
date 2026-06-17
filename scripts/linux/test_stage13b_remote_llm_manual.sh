#!/usr/bin/env bash
# Stage 13B: Manual Remote LLM Verification Script
# This script verifies that run-eino works with a real or mock LLM endpoint.
# It is NOT part of the default CI/VM E2E; it requires manual setup.
#
# Usage:
#   export EINO_LLM_ENABLED=true
#   export EINO_LLM_PROVIDER=openai_compatible
#   export EINO_LLM_ENDPOINT=http://127.0.0.1:8800/v1/chat/completions
#   export EINO_LLM_MODEL=gpt-4
#   export EINO_LLM_API_KEY=sk-placeholder
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
  printf '  This is expected when using the mock server.\n'
  printf '  But run-eino will fall back to deterministic-stub.\n'
fi

if [ -z "${EINO_LLM_ENDPOINT:-}" ]; then
  printf 'WARNING: EINO_LLM_ENDPOINT is not set.\n'
fi

printf 'Configuration:\n'
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
  printf '  Start the agent first: bash start_all.sh\n'
  exit 1
fi
printf '  PASS: Go Agent is running.\n\n'

# Test 2: Call run-eino with SSH anomaly task.
printf '2. Calling run-eino with: check SSH login anomaly\n'
RESPONSE=$(curl -s -f -X POST "$KYLIN_GUARD_AGENT_URL/api/agent/run-eino" \
  -H "Content-Type: application/json" \
  -d '{"task":"check SSH login anomaly"}' 2>&1 || true)

if [ -z "$RESPONSE" ]; then
  printf '  FAIL: Empty response from run-eino\n'
  exit 1
fi

# Use python3 to validate the response.
python3 - "$RESPONSE" <<'PYCHECK'
import json, sys

resp = json.loads(sys.argv[1])
errors = []

def check(cond, msg):
    if not cond:
        errors.append(msg)

# Response structure
check("task" in resp, "response missing 'task'")
check("decision" in resp, "response missing 'decision'")
check(resp.get("decision") != "", "decision is empty")

# Security report
report = resp.get("security_report") or {}
check(report.get("title", "") != "", "security_report.title is empty")
check(report.get("route") == "eino-runtime", f"expected route=eino-runtime, got {report.get('route')}")
check(report.get("runtime") == "eino", f"expected runtime=eino, got {report.get('runtime')}")

# Metadata
meta = report.get("audit_metadata") or {}
check(meta.get("llm_enabled") is not None, "llm_enabled missing in metadata")
llm_enabled = meta.get("llm_enabled", False)
check(meta.get("chat_model_adapter") == "interface-v1", "expected chat_model_adapter=interface-v1")
check(meta.get("eino_runtime_version") == "stage13a-v1", "expected stage13a-v1")

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
            if "bearer " in vl or "sk-" in vl[:10] or "-----begin" in vl:
                check(False, f"sensitive value in reasoning_trace span '{span.get('type')}' attr '{k}'")

# Check that API key is not leaked in response body
resp_str = json.dumps(resp)
if "sk-" in resp_str and "test" not in resp_str.lower():
    check(False, "possible API key leak in response body")
if "Authorization" in resp_str and "Bearer" in resp_str:
    check(False, "possible Authorization header leak in response body")

if errors:
    print("FAILURES:")
    for e in errors:
        print(f"  - {e}")
    raise SystemExit(1)
else:
    print("  PASS: All checks passed.")
    print(f"  decision={resp.get('decision','?')}")
    print(f"  llm_enabled={llm_enabled}")
    print(f"  chat_model={meta.get('chat_model','?')}")
    if meta.get("remote_llm_used") is not None:
        print(f"  remote_llm_used={meta.get('remote_llm_used')}")
    if meta.get("fallback_used") is not None:
        print(f"  fallback_used={meta.get('fallback_used')}")
    if meta.get("fallback_reason"):
        print(f"  fallback_reason={meta.get('fallback_reason')}")
    span_types = [s["type"] for s in spans]
    print(f"  span_types={span_types}")

print("")
print("Stage 13B remote LLM manual check passed.")
PYCHECK
