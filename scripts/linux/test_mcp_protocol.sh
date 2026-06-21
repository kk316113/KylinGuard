#!/usr/bin/env bash
set -euo pipefail

MCP_URL="${MCP_URL:-http://127.0.0.1:8080/mcp}"

python3 - "$MCP_URL" <<'PY'
import json
import sys
import urllib.request

url = sys.argv[1]


def call(request_id, method, params):
    payload = json.dumps({
        "jsonrpc": "2.0",
        "id": request_id,
        "method": method,
        "params": params,
    }).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=payload,
        headers={
            "Content-Type": "application/json",
            "Accept": "application/json, text/event-stream",
            "Mcp-Protocol-Version": "2025-06-18",
        },
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=20) as response:
        body = response.read().decode("utf-8")
        content_type = response.headers.get("Content-Type", "")
    if "text/event-stream" in content_type:
        data_lines = [line[5:].strip() for line in body.splitlines() if line.startswith("data:")]
        if not data_lines:
            raise AssertionError(f"{method} returned SSE without a data event")
        message = json.loads(data_lines[-1])
    else:
        message = json.loads(body)
    if message.get("error"):
        raise AssertionError(f"{method} returned MCP error: {message['error']}")
    return message.get("result") or {}


def require(condition, message):
    if not condition:
        raise AssertionError(message)


initialized = call(1, "initialize", {
    "protocolVersion": "2025-06-18",
    "capabilities": {},
    "clientInfo": {"name": "kylin-v11-acceptance", "version": "1.0"},
})
require(initialized.get("protocolVersion") == "2025-06-18", "unexpected MCP protocol version")
require((initialized.get("serverInfo") or {}).get("name") == "kylin-guard-mcp", "unexpected MCP server identity")
print("MCP initialize: PASS")

listed = call(2, "tools/list", {})
tools = listed.get("tools") or []
by_name = {tool.get("name"): tool for tool in tools}
for required in ("process_inspector", "open_file_inspector", "disk_io_checker"):
    require(required in by_name, f"MCP tool list missing {required}")
    annotations = by_name[required].get("annotations") or {}
    require(annotations.get("readOnlyHint") is True, f"{required} is not marked read-only")
    require(annotations.get("destructiveHint") is False, f"{required} is marked destructive")
require("safe_shell" not in by_name, "safe_shell must not be published through MCP")
print(f"MCP tools/list: PASS ({len(tools)} tools)")

allowed = call(3, "tools/call", {
    "name": "disk_io_checker",
    "arguments": {"sample_ms": 100},
})
allowed_content = allowed.get("structuredContent") or {}
require(allowed.get("isError") is not True, "read-only disk I/O call returned MCP error")
require(allowed_content.get("status") == "ok", "disk I/O tool did not complete")
trace = allowed_content.get("trace") or {}
execution = trace.get("execution_context") or {}
require(trace.get("allowed_by_policy") is True, "allowed MCP trace lacks policy approval")
require(execution.get("executor") == "least_privilege_proxy", "allowed MCP call bypassed Exec Proxy")
require(execution.get("shell_used") is False and execution.get("sudo_used") is False,
        "allowed MCP call used shell or sudo")
print("MCP allowed tools/call: PASS")

denied = call(4, "tools/call", {
    "name": "open_file_inspector",
    "arguments": {"path": "/etc/shadow"},
})
denied_content = denied.get("structuredContent") or {}
denied_audit = denied_content.get("audit_result") or {}
denied_trace = denied_content.get("trace") or {}
require(denied.get("isError") is True, "policy denial is not marked as an MCP tool error")
require(denied_content.get("status") == "denied", "sensitive path was not denied")
require(denied_audit.get("decision") == "deny", "policy denial lost authoritative deny decision")
require(denied_audit.get("method") == "tool_policy", "policy denial audit method is incorrect")
require(denied_trace.get("allowed_by_policy") is False, "denied trace is marked policy-allowed")
require(not denied_content.get("output"), "denied MCP call returned tool output")
print("MCP denied tools/call: PASS")

print("KylinGuard standard MCP acceptance: PASS")
PY
