#!/usr/bin/env bash
set -euo pipefail

AGENT_BASE_URL="${AGENT_BASE_URL:-http://127.0.0.1:8080}"
TEMP_FILE="/tmp/kylinguard-lsof-acceptance.$$"
HOLDER_PID=""

cleanup() {
  if [[ -n "$HOLDER_PID" ]]; then
    kill "$HOLDER_PID" >/dev/null 2>&1 || true
    wait "$HOLDER_PID" 2>/dev/null || true
  fi
  rm -f -- "$TEMP_FILE"
}
trap cleanup EXIT

if ! command -v lsof >/dev/null 2>&1; then
  printf 'lsof is required for A2 OS sensing acceptance\n' >&2
  exit 1
fi

printf 'KylinGuard lsof acceptance fixture\n' >"$TEMP_FILE"
tail -f "$TEMP_FILE" >/dev/null 2>&1 &
HOLDER_PID=$!
sleep 0.2

python3 - "$AGENT_BASE_URL" "$TEMP_FILE" "$HOLDER_PID" <<'PY'
import json
import sys
import urllib.error
import urllib.request

base_url = sys.argv[1].rstrip("/")
fixture_path = sys.argv[2]
holder_pid = int(sys.argv[3])


def post(payload):
    request = urllib.request.Request(
        base_url + "/api/tools/call",
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=20) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        detail = error.read(512).decode("utf-8", errors="replace")
        raise AssertionError(f"tool call returned HTTP {error.code}: {detail}") from error


def require(condition, message):
    if not condition:
        raise AssertionError(message)


open_files = post({
    "tool_name": "open_file_inspector",
    "input": {"path": fixture_path, "limit": 20},
    "reason": "A2 lsof real process/file ownership acceptance",
})
require(open_files.get("status") == "ok", f"lsof tool failed: {open_files.get('message')}")
records = open_files.get("output", {}).get("open_files", [])
require(any(item.get("pid") == holder_pid and item.get("name") == fixture_path for item in records),
        f"lsof did not find holder pid {holder_pid}")
require(open_files.get("trace", {}).get("execution_context", {}).get("executor") == "least_privilege_proxy",
        "lsof did not execute through Exec Proxy")
print("real lsof file holder detection: PASS")

zombies = post({
    "tool_name": "process_inspector",
    "input": {"state": "ZOMBIE", "limit": 100},
    "reason": "A2 zombie process sensing acceptance",
})
require(zombies.get("status") == "ok", f"process tool failed: {zombies.get('message')}")
process_output = zombies.get("output", {})
require(isinstance(process_output.get("zombie_count"), int), "zombie_count is missing")
require(process_output.get("risk_level") in {"low", "medium", "high"}, "process risk level is missing")
print("zombie process sensing: PASS")

disk_io = post({
    "tool_name": "disk_io_checker",
    "input": {"sample_ms": 500},
    "reason": "A2 disk I/O sensing acceptance",
})
require(disk_io.get("status") == "ok", f"disk I/O tool failed: {disk_io.get('message')}")
disk_output = disk_io.get("output", {})
require(disk_output.get("source") == "procfs:/proc/diskstats", "diskstats source is missing")
require(len(disk_output.get("devices", [])) > 0, "no physical disk metrics were returned")
require(disk_output.get("risk_level") in {"low", "medium", "high"}, "disk I/O risk level is missing")
print("disk I/O live sampling: PASS")

denied = post({
    "tool_name": "open_file_inspector",
    "input": {"path": "/etc/shadow"},
    "reason": "sensitive path denial acceptance",
})
require(denied.get("status") == "denied", "sensitive lsof path was not denied")
require(not denied.get("output"), "denied lsof path returned tool output")
print("sensitive lsof path denial: PASS")

print("KylinGuard A2 OS sensing acceptance: PASS")
PY
