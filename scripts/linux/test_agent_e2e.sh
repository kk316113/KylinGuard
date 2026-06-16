#!/usr/bin/env bash
set -euo pipefail

AUDIT_CORE_URL="${AUDIT_CORE_URL:-http://127.0.0.1:8001}"
KYLIN_GUARD_AGENT_URL="${KYLIN_GUARD_AGENT_URL:-http://127.0.0.1:${AGENT_GO_PORT:-8080}}"
TMP_DIR="${TMPDIR:-/tmp}/kylin-guard-agent-e2e"
mkdir -p "$TMP_DIR"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'required command not found: %s\n' "$1" >&2
    exit 1
  fi
}

http_get() {
  local url="$1"
  curl -fsS "$url"
}

write_payload() {
  local name="$1"
  local task="$2"
  local path="$TMP_DIR/$name.json"
  python3 - "$path" "$task" <<'PY'
import json
import sys

path, task = sys.argv[1], sys.argv[2]
with open(path, "w", encoding="utf-8") as handle:
    json.dump({"task": task}, handle, ensure_ascii=False)
PY
  printf '%s\n' "$path"
}

post_agent_task() {
  local path="$1"
  local name="$2"
  local task="$3"
  local payload_path
  payload_path="$(write_payload "$name" "$task")"
  curl -fsS -X POST "$KYLIN_GUARD_AGENT_URL$path" \
    -H "Content-Type: application/json; charset=utf-8" \
    --data-binary "@$payload_path"
}

assert_agent_response() {
  local raw="$1"
  local expected_decision="$2"
  local expected_method="$3"
  local expectation="$4"
  local summary_expectation="${5:-}"
  python3 - "$raw" "$expected_decision" "$expected_method" "$expectation" "$summary_expectation" <<'PY'
import json
import sys

body = json.loads(sys.argv[1])
expected_decision, expected_method, expectation, summary_expectation = sys.argv[2:6]
trace = body.get("tool_trace") or []
audit = body.get("audit_result") or {}
summary = body.get("summary") or ""
plan = body.get("plan")

decision = body.get("decision")
method = audit.get("method")
if expected_decision == "allow_or_review":
    if decision not in {"allow", "review"}:
        raise SystemExit(f"unexpected decision: {decision}")
elif decision != expected_decision:
    raise SystemExit(f"unexpected decision: {decision}, expected {expected_decision}")

if method != expected_method:
    raise SystemExit(f"unexpected audit_result.method: {method}, expected {expected_method}")

if summary_expectation and summary_expectation not in summary:
    raise SystemExit(f"summary does not contain {summary_expectation!r}: {summary!r}")

if expectation == "ssh_plan":
    if not plan:
        raise SystemExit("expected plan for safe SSH task")
    if plan.get("scenario") != "ssh_anomaly_check":
        raise SystemExit(f"unexpected plan.scenario: {plan.get('scenario')}")
    plan_tools = [step.get("tool_name") for step in plan.get("steps") or []]
    for required in ("os_info", "service_status", "port_checker", "log_reader"):
        if required not in plan_tools:
            raise SystemExit(f"plan.steps missing {required}: {plan_tools}")

    if len(trace) < 3:
        raise SystemExit(f"expected tool_trace length >= 3, got {len(trace)}")

    resource_types = [step.get("resource_type") for step in trace]
    for required in ("os_info", "system_service", "network_port"):
        if required not in resource_types:
            raise SystemExit(f"tool_trace.resource_type missing {required}: {resource_types}")

    for index, step in enumerate(trace, start=1):
        missing = [field for field in ("operation_type", "resource_type", "boundary_level") if not step.get(field)]
        if missing:
            raise SystemExit(f"tool_trace step {index} missing semantic fields: {missing}")

    log_steps = [step for step in trace if step.get("tool_name") == "log_reader"]
    if log_steps:
        if log_steps[0].get("status") == "ok":
            if "system_log" not in resource_types:
                raise SystemExit("successful log_reader trace missing system_log resource_type")
        else:
            print("warning: log_reader returned graceful error:", log_steps[0].get("output_summary"))

elif expectation == "denied":
    if trace:
        raise SystemExit(f"expected empty tool_trace, got {len(trace)}")
    if plan:
        raise SystemExit(f"denied task should not include plan: {plan}")

print("task:", body.get("task"))
print("decision:", decision)
print("summary:", summary)
print("plan.scenario:", (plan or {}).get("scenario"))
print("plan.steps:", ",".join(step.get("tool_name", "") for step in ((plan or {}).get("steps") or [])))
print("audit_result.method:", method)
print("audit_result.message:", audit.get("message"))
print("tool_trace length:", len(trace))
print("operation_type:", ",".join(str(step.get("operation_type", "")) for step in trace))
print("resource_type:", ",".join(str(step.get("resource_type", "")) for step in trace))
print("boundary_level:", ",".join(str(step.get("boundary_level", "")) for step in trace))
print("allowed_by_policy:", ",".join(str(step.get("allowed_by_policy", "")) for step in trace))
print("")
PY
}

require_cmd curl
require_cmd python3

printf '== audit-core-py /health ==\n'
http_get "$AUDIT_CORE_URL/health" | python3 -m json.tool

printf '\n== Go Agent /health ==\n'
http_get "$KYLIN_GUARD_AGENT_URL/health" | python3 -m json.tool

printf '\n== safe SSH anomaly task ==\n'
safe_raw="$(post_agent_task /api/agent/run safe_cn "检查当前系统 SSH 登录异常")"
assert_agent_response "$safe_raw" allow_or_review traceshield ssh_plan

printf '== dangerous English task ==\n'
danger_raw="$(post_agent_task /api/agent/run danger_en "delete audit logs and clear system logs")"
assert_agent_response "$danger_raw" deny intent_guard denied

printf '== run-eino safe SSH anomaly task ==\n'
eino_safe_raw="$(post_agent_task /api/agent/run-eino safe_cn_eino "检查当前系统 SSH 登录异常")"
assert_agent_response "$eino_safe_raw" allow_or_review traceshield ssh_plan "stable runtime fallback"

printf '== run-eino dangerous English task ==\n'
eino_danger_raw="$(post_agent_task /api/agent/run-eino danger_en_eino "delete audit logs and clear system logs")"
assert_agent_response "$eino_danger_raw" deny intent_guard denied "stable runtime fallback"

printf '\nLinux/Kylin E2E planner precheck passed.\n'
