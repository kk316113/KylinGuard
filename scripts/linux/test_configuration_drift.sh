#!/usr/bin/env bash
set -euo pipefail

AGENT_BASE_URL="${AGENT_BASE_URL:-http://127.0.0.1:8080}"
PACKAGE_NAME="${RPM_VERIFY_PACKAGE:-systemd}"

python3 - "$AGENT_BASE_URL" "$PACKAGE_NAME" <<'PY'
import json
import sys
import urllib.request

base_url, package_name = sys.argv[1].rstrip("/"), sys.argv[2]


def post(payload):
    request = urllib.request.Request(
        base_url + "/api/tools/call",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.load(response)


result = post({
    "tool_name": "configuration_drift_detector",
    "input": {"packages": [package_name]},
    "reason": "read-only RPM configuration drift acceptance",
})
assert result.get("status") == "ok", result
output = result.get("output", {})
assert output.get("baseline_source") == "rpm_package_database", output
assert isinstance(output.get("findings"), list), output
context = result.get("trace", {}).get("execution_context", {})
assert context.get("executor") == "least_privilege_proxy", context
assert context.get("command_name") == "rpm", context
assert context.get("shell_used") is False and context.get("sudo_used") is False, context

denied = post({
    "tool_name": "configuration_drift_detector",
    "input": {"packages": ["--all"]},
    "reason": "RPM option injection denial acceptance",
})
assert denied.get("status") == "denied", denied
assert not denied.get("output"), denied
print("KylinGuard configuration drift acceptance: PASS")
PY
