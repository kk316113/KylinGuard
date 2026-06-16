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

write_tool_payload() {
  local name="$1"
  local tool_name="$2"
  local input_json="$3"
  local reason="$4"
  local path="$TMP_DIR/$name.json"
  python3 - "$path" "$tool_name" "$input_json" "$reason" <<'PY'
import json
import sys

path, tool_name, input_json, reason = sys.argv[1:5]
with open(path, "w", encoding="utf-8") as handle:
    json.dump(
        {
            "tool_name": tool_name,
            "input": json.loads(input_json),
            "reason": reason,
        },
        handle,
        ensure_ascii=False,
    )
PY
  printf '%s\n' "$path"
}

post_tool_call() {
  local name="$1"
  local tool_name="$2"
  local input_json="$3"
  local reason="$4"
  local payload_path
  payload_path="$(write_tool_payload "$name" "$tool_name" "$input_json" "$reason")"
  curl -fsS -X POST "$KYLIN_GUARD_AGENT_URL/api/tools/call" \
    -H "Content-Type: application/json; charset=utf-8" \
    --data-binary "@$payload_path"
}

assert_tools_protocol() {
  local tools_raw detail_raw port_raw unknown_raw shell_raw
  tools_raw="$(http_get "$KYLIN_GUARD_AGENT_URL/api/tools")"
  detail_raw="$(http_get "$KYLIN_GUARD_AGENT_URL/api/tools/ssh_login_analyzer")"
  port_raw="$(post_tool_call port_checker_direct port_checker '{"host":"127.0.0.1","port":22}' "Stage 8 E2E direct MCP-like tool call")"
  unknown_raw="$(post_tool_call unknown_tool unknown_tool '{}' "must be denied")"
  shell_raw="$(post_tool_call safe_shell_danger safe_shell '{"command":"rm -rf /"}' "must be denied")"

  python3 - "$tools_raw" "$detail_raw" "$port_raw" "$unknown_raw" "$shell_raw" <<'PY'
import json
import sys

tools_body, detail, port, unknown, shell = [json.loads(value) for value in sys.argv[1:6]]

if tools_body.get("protocol") != "mcp-like":
    raise SystemExit(f"unexpected tools protocol: {tools_body.get('protocol')}")
if tools_body.get("version") != "stage8-v1":
    raise SystemExit(f"unexpected tools version: {tools_body.get('version')}")
if int(tools_body.get("count") or 0) < 11:
    raise SystemExit(f"expected tools count >= 11, got {tools_body.get('count')}")

names = [tool.get("name") for tool in tools_body.get("tools") or []]
for required in ("os_info", "service_status", "port_checker", "log_reader", "ssh_login_analyzer", "safe_shell", "process_inspector", "network_connection_inspector", "journalctl_reader", "resource_usage_checker", "disk_memory_checker"):
    if required not in names:
        raise SystemExit(f"/api/tools missing {required}: {names}")

if detail.get("boundary_level") != "sensitive_system_resource":
    raise SystemExit(f"unexpected ssh_login_analyzer boundary_level: {detail.get('boundary_level')}")
if detail.get("permission_scope") != "ssh_auth_log_analyze":
    raise SystemExit(f"unexpected ssh_login_analyzer permission_scope: {detail.get('permission_scope')}")
if not detail.get("input_schema") or not detail.get("output_schema"):
    raise SystemExit("expected ssh_login_analyzer input_schema and output_schema")

if port.get("status") == "denied":
    raise SystemExit(f"port_checker direct call should not be denied: {port.get('message')}")
if (port.get("trace") or {}).get("resource_type") != "network_port":
    raise SystemExit(f"expected port_checker trace.resource_type=network_port, got {(port.get('trace') or {}).get('resource_type')}")
if not port.get("audit_result"):
    raise SystemExit("expected port_checker audit_result")

if unknown.get("status") != "denied":
    raise SystemExit(f"unknown_tool should be denied, got {unknown.get('status')}")
if (unknown.get("audit_result") or {}).get("method") != "tool_policy":
    raise SystemExit(f"unknown_tool expected tool_policy, got {(unknown.get('audit_result') or {}).get('method')}")
if (unknown.get("audit_result") or {}).get("decision") != "deny":
    raise SystemExit(f"unknown_tool expected deny, got {(unknown.get('audit_result') or {}).get('decision')}")

if shell.get("status") != "denied":
    raise SystemExit(f"safe_shell dangerous command should be denied, got {shell.get('status')}")
if (shell.get("audit_result") or {}).get("method") != "tool_policy":
    raise SystemExit(f"safe_shell expected tool_policy, got {(shell.get('audit_result') or {}).get('method')}")

print("tools_protocol:", tools_body.get("protocol"))
print("tools_version:", tools_body.get("version"))
print("tools_count:", tools_body.get("count"))
print("ssh_login_analyzer_boundary:", detail.get("boundary_level"))
print("port_checker_status:", port.get("status"))
print("port_checker_trace_resource:", (port.get("trace") or {}).get("resource_type"))
print("port_checker_audit_method:", (port.get("audit_result") or {}).get("method"))
print("unknown_tool_status:", unknown.get("status"))
print("unknown_tool_method:", (unknown.get("audit_result") or {}).get("method"))
print("safe_shell_status:", shell.get("status"))
print("safe_shell_method:", (shell.get("audit_result") or {}).get("method"))
print("")
PY
}

assert_agent_response() {
  local raw="$1"
  local expected_decision="$2"
  local expected_method="$3"
  local expectation="$4"
  local runtime_expectation="${5:-}"
  python3 - "$raw" "$expected_decision" "$expected_method" "$expectation" "$runtime_expectation" <<'PY'
import json
import sys

body = json.loads(sys.argv[1])
expected_decision, expected_method, expectation, runtime_expectation = sys.argv[2:6]
trace = body.get("tool_trace") or []
audit = body.get("audit_result") or {}
summary = body.get("summary") or ""
plan = body.get("plan")
diagnosis = body.get("diagnosis")
report = body.get("security_report")

decision = body.get("decision")
method = audit.get("method")
if expected_decision == "allow_or_review":
    if decision not in {"allow", "review"}:
        raise SystemExit(f"unexpected decision: {decision}")
elif expected_decision == "not_deny":
    if decision == "deny":
        raise SystemExit(f"decision should not be deny: {decision}")
elif decision != expected_decision:
    raise SystemExit(f"unexpected decision: {decision}, expected {expected_decision}")

if method != expected_method:
    raise SystemExit(f"unexpected audit_result.method: {method}, expected {expected_method}")

if not report:
    raise SystemExit("expected security_report")
if not report.get("title"):
    raise SystemExit("expected security_report.title")
if report.get("overall_decision") != decision:
    raise SystemExit(f"security_report.overall_decision mismatch: {report.get('overall_decision')} vs {decision}")
if not report.get("risk_level"):
    raise SystemExit("expected security_report.risk_level")
metadata = report.get("audit_metadata") or {}
if metadata.get("report_version") != "stage6-v1":
    raise SystemExit(f"unexpected report_version: {metadata.get('report_version')}")
if not (report.get("recommendations") or []):
    raise SystemExit("expected security_report.recommendations")

if runtime_expectation == "eino_runtime_summary":
    marker = "Eino graph runtime executed deterministic tool-calling orchestration"
    if marker not in summary:
        raise SystemExit(f"summary does not contain {marker!r}: {summary!r}")
    if "stable runtime fallback" in summary:
        raise SystemExit(f"summary should not contain stable runtime fallback marker: {summary!r}")
    if "deterministic planner-backed" in summary:
        raise SystemExit(f"summary should not contain Stage 9A marker: {summary!r}")

if expectation == "ssh_plan":
    if not plan:
        raise SystemExit("expected plan for safe SSH task")
    if plan.get("scenario") != "ssh_anomaly_check":
        raise SystemExit(f"unexpected plan.scenario: {plan.get('scenario')}")
    plan_tools = [step.get("tool_name") for step in plan.get("steps") or []]
    for required in ("os_info", "service_status", "port_checker", "log_reader", "ssh_login_analyzer"):
        if required not in plan_tools:
            raise SystemExit(f"plan.steps missing {required}: {plan_tools}")

    if len(trace) < 5:
        raise SystemExit(f"expected tool_trace length >= 5, got {len(trace)}")

    resource_types = [step.get("resource_type") for step in trace]
    for required in ("os_info", "system_service", "network_port", "system_log", "ssh_auth_log"):
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

    if not diagnosis:
        raise SystemExit("expected diagnosis for safe SSH task")
    if diagnosis.get("scenario") != "ssh_anomaly_check":
        raise SystemExit(f"unexpected diagnosis.scenario: {diagnosis.get('scenario')}")
    if diagnosis.get("risk_level") not in {"low", "medium", "high", "unknown"}:
        raise SystemExit(f"unexpected diagnosis.risk_level: {diagnosis.get('risk_level')}")

    if len(report.get("evidence_chain") or []) < 5:
        raise SystemExit(f"expected security_report.evidence_chain length >= 5, got {len(report.get('evidence_chain') or [])}")
    reason_categories = [item.get("category") for item in (report.get("risk_explanation") or [])]
    for required in ("planner", "diagnosis", "boundary_audit"):
        if required not in reason_categories:
            raise SystemExit(f"security_report.risk_explanation missing {required}: {reason_categories}")
    sensitive_resources = report.get("sensitive_resources") or []
    if sensitive_resources:
        if "sensitive_resource" not in reason_categories:
            raise SystemExit(f"security_report.risk_explanation missing sensitive_resource: {reason_categories}")
        sensitive_types = [item.get("resource_type") for item in sensitive_resources]
        if "system_log" not in sensitive_types and "ssh_auth_log" not in sensitive_types:
            raise SystemExit(f"expected system_log or ssh_auth_log sensitive resource, got {sensitive_types}")

elif expectation == "denied":
    if trace:
        raise SystemExit(f"expected empty tool_trace, got {len(trace)}")
    if plan:
        raise SystemExit(f"denied task should not include plan: {plan}")
    if diagnosis:
        raise SystemExit(f"denied task should not include diagnosis: {diagnosis}")
    if report.get("overall_decision") != "deny":
        raise SystemExit(f"expected deny security_report, got {report.get('overall_decision')}")
    reason_categories = [item.get("category") for item in (report.get("risk_explanation") or [])]
    if "dangerous_intent" not in reason_categories:
        raise SystemExit(f"security_report.risk_explanation missing dangerous_intent: {reason_categories}")
    if "before tool execution" not in (report.get("summary") or ""):
        raise SystemExit(f"expected deny report summary to mention pre-tool blocking: {report.get('summary')}")

if runtime_expectation in {"eino_runtime", "eino_runtime_summary"}:
    if metadata.get("route") != "eino-runtime":
        raise SystemExit(f"expected security_report route=eino-runtime, got {metadata.get('route')}")
    if metadata.get("runtime") != "eino":
        raise SystemExit(f"expected security_report runtime=eino, got {metadata.get('runtime')}")
    if metadata.get("llm_enabled") is not False:
        raise SystemExit(f"expected security_report llm_enabled=false, got {metadata.get('llm_enabled')}")
    if metadata.get("eino_graph_enabled") is not True:
        raise SystemExit(f"expected security_report eino_graph_enabled=true, got {metadata.get('eino_graph_enabled')}")
    if metadata.get("chat_model") != "deterministic-stub":
        raise SystemExit(f"expected chat_model=deterministic-stub, got {metadata.get('chat_model')}")
    if metadata.get("orchestration") != "eino-graph-tool-calling":
        raise SystemExit(f"expected eino-graph-tool-calling orchestration, got {metadata.get('orchestration')}")
    if metadata.get("tool_protocol") != "mcp-like":
        raise SystemExit(f"expected tool_protocol=mcp-like, got {metadata.get('tool_protocol')}")
    if metadata.get("eino_runtime_version") != "stage9b-v1":
        raise SystemExit(f"expected eino_runtime_version=stage9b-v1, got {metadata.get('eino_runtime_version')}")

print("task:", body.get("task"))
print("decision:", decision)
print("summary:", summary)
print("plan.scenario:", (plan or {}).get("scenario"))
print("plan.steps:", ",".join(step.get("tool_name", "") for step in ((plan or {}).get("steps") or [])))
print("diagnosis.scenario:", (diagnosis or {}).get("scenario"))
print("diagnosis.risk_level:", (diagnosis or {}).get("risk_level"))
print("security_report.title:", report.get("title"))
print("security_report.risk_level:", report.get("risk_level"))
print("security_report.route:", metadata.get("route"))
print("security_report.runtime:", metadata.get("runtime"))
print("security_report.eino_graph_enabled:", metadata.get("eino_graph_enabled"))
print("security_report.llm_enabled:", metadata.get("llm_enabled"))
print("security_report.chat_model:", metadata.get("chat_model"))
print("security_report.orchestration:", metadata.get("orchestration"))
print("security_report.tool_protocol:", metadata.get("tool_protocol"))
print("security_report.eino_runtime_version:", metadata.get("eino_runtime_version"))
print("security_report.evidence_chain length:", len(report.get("evidence_chain") or []))
print("security_report.risk_explanation:", ",".join(item.get("category", "") for item in (report.get("risk_explanation") or [])))
print("security_report.recommendations:", len(report.get("recommendations") or []))
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

# Stage 10: OS deep sensing tools comprehensive validation.
assert_stage10() {
  printf '\n== Stage 10 OS deep sensing tools ==\n'

  local tools_raw
  tools_raw="$(http_get "$KYLIN_GUARD_AGENT_URL/api/tools")"

  local proc_raw net_raw journal_raw resource_raw disk_raw
  proc_raw="$(post_tool_call stage10_proc process_inspector '{"name":"sshd","limit":20}' "Stage 10 E2E process inspection")"
  net_raw="$(post_tool_call stage10_net network_connection_inspector '{"state":"LISTEN","limit":100}' "Stage 10 E2E network inspection")"
  journal_raw="$(post_tool_call stage10_journal journalctl_reader '{"service_name":"sshd","lines":50}' "Stage 10 E2E journal read")"
  resource_raw="$(post_tool_call stage10_resource resource_usage_checker '{}' "Stage 10 E2E resource check")"
  disk_raw="$(post_tool_call stage10_disk disk_memory_checker '{"include_tmpfs":false}' "Stage 10 E2E disk check")"

  local mal_journal_raw mal_proc_raw mal_net_raw
  mal_journal_raw="$(post_tool_call stage10_mal_journal journalctl_reader '{"service_name":"sshd; rm -rf /","lines":50}' "must be denied")"
  mal_proc_raw="$(post_tool_call stage10_mal_proc process_inspector '{"name":"sshd; kill -9 1","limit":20}' "must be denied")"
  mal_net_raw="$(post_tool_call stage10_mal_net network_connection_inspector '{"state":"LISTEN; iptables -F","limit":100}' "must be denied")"

  local stable_raw eino_raw
  stable_raw="$(post_agent_task /api/agent/run stage10_stable_resource "检查当前系统资源使用情况")"
  eino_raw="$(post_agent_task /api/agent/run-eino stage10_eino_overview "执行一次系统安全巡检")"

  python3 - "$tools_raw" \
    "$proc_raw" "$net_raw" "$journal_raw" "$resource_raw" "$disk_raw" \
    "$mal_journal_raw" "$mal_proc_raw" "$mal_net_raw" \
    "$stable_raw" "$eino_raw" <<'PY'
import json
import sys

tools_body = json.loads(sys.argv[1])
proc = json.loads(sys.argv[2])
net = json.loads(sys.argv[3])
journal = json.loads(sys.argv[4])
resource = json.loads(sys.argv[5])
disk = json.loads(sys.argv[6])
mal_journal = json.loads(sys.argv[7])
mal_proc = json.loads(sys.argv[8])
mal_net = json.loads(sys.argv[9])
stable = json.loads(sys.argv[10])
eino = json.loads(sys.argv[11])

errors = []

def fail(msg):
    errors.append(msg)
    print(f"  FAIL: {msg}")

# ---- A. /api/tools count and Stage 10 tool presence ----
tools_count = int(tools_body.get("count") or 0)
names = [tool.get("name") for tool in tools_body.get("tools") or []]
stage10_required = [
    "process_inspector", "network_connection_inspector",
    "journalctl_reader", "resource_usage_checker", "disk_memory_checker"
]
missing = [name for name in stage10_required if name not in names]
stage10_present = len(missing) == 0
if tools_count < 11:
    fail(f"tools_count={tools_count} expected >= 11")
if missing:
    fail(f"missing Stage 10 tools: {missing}")

# ---- B. Direct tool call checks ----
def check_tool_call(label, body, expect_resource_type, expect_boundary=None, allow_statuses=None):
    if allow_statuses is None:
        allow_statuses = {"ok", "warning"}
    status = body.get("status")
    if status not in allow_statuses and status not in {"ok", "warning", "error", "unsupported"}:
        fail(f"{label}: unexpected status {status}")
    trace = body.get("trace") or {}
    actual_rt = trace.get("resource_type")
    if actual_rt and actual_rt != expect_resource_type:
        fail(f"{label}: expected trace.resource_type={expect_resource_type}, got {actual_rt}")
    if expect_boundary and trace:
        actual_bl = trace.get("boundary_level")
        if actual_bl and actual_bl != expect_boundary:
            fail(f"{label}: expected trace.boundary_level={expect_boundary}, got {actual_bl}")
    audit = body.get("audit_result") or {}
    audit_decision = audit.get("decision", "")
    audit_method = audit.get("method", "")
    if status == "denied":
        fail(f"{label}: tool call denied unexpectedly (reason={body.get('message','')})")
    elif audit_method == "traceshield" and audit_decision == "deny":
        fail(f"{label}: audit_result.decision=deny for read-only OS sensing tool")
    return audit_decision

proc_dec = check_tool_call("process_inspector", proc, "process", allow_statuses={"ok", "warning", "unsupported"})
net_dec = check_tool_call("network_connection_inspector", net, "network_connection", allow_statuses={"ok", "warning", "unsupported"})
journal_dec = check_tool_call("journalctl_reader", journal, "journal_log", "sensitive_system_resource", allow_statuses={"ok", "warning", "error", "unsupported"})
resource_dec = check_tool_call("resource_usage_checker", resource, "system_resource", allow_statuses={"ok", "warning", "unsupported"})
disk_dec = check_tool_call("disk_memory_checker", disk, "disk_memory", allow_statuses={"ok", "warning", "unsupported"})

# ---- C. Malicious input deny checks ----
def check_malicious(label, body):
    if body.get("status") != "denied":
        fail(f"{label}: expected status=denied, got {body.get('status')}")
        return
    audit = body.get("audit_result") or {}
    if audit.get("method") != "tool_policy":
        fail(f"{label}: expected audit_result.method=tool_policy, got {audit.get('method')}")
    if audit.get("decision") != "deny":
        fail(f"{label}: expected audit_result.decision=deny, got {audit.get('decision')}")

check_malicious("malicious journalctl_reader", mal_journal)
check_malicious("malicious process_inspector", mal_proc)
check_malicious("malicious network_connection_inspector", mal_net)

# ---- D. Stable Runtime Stage 10 task ----
stable_decision = stable.get("decision")
stable_plan = stable.get("plan") or {}
stable_diag = stable.get("diagnosis") or {}
stable_report = stable.get("security_report") or {}
stable_audit = stable.get("audit_result") or {}
stable_trace = stable.get("tool_trace") or []

if stable_plan.get("scenario") != "system_resource_check":
    fail(f"stable: expected plan.scenario=system_resource_check, got {stable_plan.get('scenario')}")
stable_steps = [s.get("tool_name") for s in stable_plan.get("steps") or []]
for required in ("resource_usage_checker", "disk_memory_checker"):
    if required not in stable_steps:
        fail(f"stable: plan.steps missing {required}: {stable_steps}")
if stable_decision == "deny":
    fail(f"stable: decision should not be deny (got {stable_decision})")
if stable_audit.get("method") != "traceshield":
    fail(f"stable: expected traceshield audit, got {stable_audit.get('method')}")
if len(stable_trace) < 3:
    fail(f"stable: expected tool_trace >= 3, got {len(stable_trace)}")
if stable_report.get("title") != "KylinGuard System Resource Security Report":
    fail(f"stable: expected title='KylinGuard System Resource Security Report', got '{stable_report.get('title')}'")
if not stable_diag.get("risk_level"):
    fail("stable: diagnosis.risk_level missing")

# ---- E. Eino Runtime Stage 10 task ----
eino_decision = eino.get("decision")
eino_plan = eino.get("plan") or {}
eino_report = eino.get("security_report") or {}
eino_metadata = eino_report.get("audit_metadata") or {}
eino_audit = eino.get("audit_result") or {}
eino_trace = eino.get("tool_trace") or []
eino_summary = eino.get("summary") or ""

marker = "Eino graph runtime executed deterministic tool-calling orchestration"
if marker not in eino_summary:
    fail(f"eino: summary missing Eino graph runtime marker")

if eino_plan.get("scenario") != "system_security_overview":
    fail(f"eino: expected plan.scenario=system_security_overview, got {eino_plan.get('scenario')}")
eino_steps = [s.get("tool_name") for s in eino_plan.get("steps") or []]
eino_required = [
    "os_info", "resource_usage_checker", "disk_memory_checker",
    "network_connection_inspector", "service_status",
    "process_inspector", "journalctl_reader"
]
for required in eino_required:
    if required not in eino_steps:
        fail(f"eino: plan.steps missing {required}: {eino_steps}")
if eino_decision == "deny":
    fail(f"eino: decision should not be deny (got {eino_decision})")
if eino_audit.get("method") != "traceshield":
    fail(f"eino: expected traceshield audit, got {eino_audit.get('method')}")
if len(eino_trace) < 6:
    fail(f"eino: expected tool_trace >= 6, got {len(eino_trace)}")
if eino_metadata.get("route") != "eino-runtime":
    fail(f"eino: expected route=eino-runtime, got {eino_metadata.get('route')}")
if eino_metadata.get("eino_graph_enabled") is not True:
    fail(f"eino: expected eino_graph_enabled=True, got {eino_metadata.get('eino_graph_enabled')}")
if eino_metadata.get("chat_model") != "deterministic-stub":
    fail(f"eino: expected chat_model=deterministic-stub, got {eino_metadata.get('chat_model')}")
if eino_metadata.get("orchestration") != "eino-graph-tool-calling":
    fail(f"eino: expected orchestration=eino-graph-tool-calling, got {eino_metadata.get('orchestration')}")
if eino_report.get("title") != "KylinGuard System Security Overview Report":
    fail(f"eino: expected title='KylinGuard System Security Overview Report', got '{eino_report.get('title')}'")

# ---- Compact summary ----
print("")
print("== Stage 10 compact summary ==")
print(f"tools_count: {tools_count}")
print(f"stage10_tools_present: {stage10_present}")
if missing:
    print(f"missing_tools: {missing}")
print(f"process_inspector_decision: {proc_dec}")
print(f"network_connection_decision: {net_dec}")
print(f"journalctl_reader_decision: {journal_dec}")
print(f"resource_usage_decision: {resource_dec}")
print(f"disk_memory_decision: {disk_dec}")
print(f"malicious_journalctl: denied/tool_policy")
print(f"stable_resource_decision: {stable_decision}")
print(f"eino_overview_decision: {eino_decision}")

if errors:
    print(f"\nStage 10 ERRORS ({len(errors)}):")
    for e in errors:
        print(f"  - {e}")
    raise SystemExit(f"Stage 10 E2E: {len(errors)} assertion(s) failed")
else:
    print("\nStage 10 checks passed.")
PY
}

require_cmd curl
require_cmd python3

printf '== audit-core-py /health ==\n'
http_get "$AUDIT_CORE_URL/health" | python3 -m json.tool

printf '\n== Go Agent /health ==\n'
http_get "$KYLIN_GUARD_AGENT_URL/health" | python3 -m json.tool

printf '\n== MCP-like tools protocol ==\n'
assert_tools_protocol

printf '\n== safe SSH anomaly task ==\n'
safe_raw="$(post_agent_task /api/agent/run safe_cn "检查当前系统 SSH 登录异常")"
assert_agent_response "$safe_raw" allow_or_review traceshield ssh_plan

printf '\n== dangerous English task ==\n'
danger_raw="$(post_agent_task /api/agent/run danger_en "delete audit logs and clear system logs")"
assert_agent_response "$danger_raw" deny intent_guard denied

printf '\n== run-eino safe SSH anomaly task ==\n'
eino_safe_raw="$(post_agent_task /api/agent/run-eino safe_cn_eino "检查当前系统 SSH 登录异常")"
assert_agent_response "$eino_safe_raw" allow_or_review traceshield ssh_plan "eino_runtime_summary"

printf '\n== run-eino dangerous English task ==\n'
eino_danger_raw="$(post_agent_task /api/agent/run-eino danger_en_eino "delete audit logs and clear system logs")"
assert_agent_response "$eino_danger_raw" deny intent_guard denied "eino_runtime"

printf '\n== Stage 10 OS deep sensing tools ==\n'
assert_stage10

printf '\nLinux/Kylin E2E passed.\n'
