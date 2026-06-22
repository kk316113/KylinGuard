#!/usr/bin/env bash
set -euo pipefail

wait_active() {
  local unit="$1"
  local attempt
  for attempt in $(seq 1 30); do
    if systemctl is-active --quiet "$unit"; then
      printf '%s: active\n' "$unit"
      return 0
    fi
    sleep 1
  done
  systemctl status --no-pager --full "$unit" >&2 || true
  return 1
}

wait_url() {
  local url="$1"
  local attempt
  for attempt in $(seq 1 30); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      printf '%s: ready\n' "$url"
      return 0
    fi
    sleep 1
  done
  printf '%s: not ready after 30 seconds\n' "$url" >&2
  return 1
}

for unit in kylin-guard-audit.service kylin-guard-agent.service kylin-guard-web.service; do
  wait_active "$unit"
done

[[ "$(systemctl show --property=User --value kylin-guard-agent.service)" == "kylinguard" ]]
[[ "$(systemctl show --property=User --value kylin-guard-audit.service)" == "kylinguard-audit" ]]
[[ "$(systemctl show --property=User --value kylin-guard-web.service)" == "kylinguard-web" ]]

wait_url http://127.0.0.1:8001/health
wait_url http://127.0.0.1:8080/health
wait_url http://127.0.0.1:5173/
python3 - <<'PY'
import json
import urllib.request

with urllib.request.urlopen("http://127.0.0.1:8080/api/agent/capabilities", timeout=5) as response:
    capabilities = json.load(response)
names = {tool["tool_name"] for tool in capabilities["available_tools"]}
assert "configuration_drift_detector" in names
assert "safe_shell" not in names
PY
printf 'KylinGuard stack health: PASS\n'
