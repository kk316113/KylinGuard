#!/usr/bin/env bash
set -euo pipefail

for unit in kylin-guard-audit.service kylin-guard-agent.service kylin-guard-web.service; do
  systemctl is-active --quiet "$unit" || { systemctl status --no-pager "$unit" >&2; exit 1; }
  printf '%s: active\n' "$unit"
done

[[ "$(systemctl show --property=User --value kylin-guard-agent.service)" == "kylinguard" ]]
[[ "$(systemctl show --property=User --value kylin-guard-audit.service)" == "kylinguard-audit" ]]
[[ "$(systemctl show --property=User --value kylin-guard-web.service)" == "kylinguard-web" ]]

curl -fsS http://127.0.0.1:8001/health >/dev/null
curl -fsS http://127.0.0.1:8080/health >/dev/null
curl -fsS http://127.0.0.1:5173/ >/dev/null
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
