#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
INSTALL_ROOT="/opt/kylin-guard"
CONFIG_ROOT="/etc/kylin-guard"

if [[ "${EUID}" -ne 0 ]]; then
  printf 'run this installer as root\n' >&2
  exit 1
fi
for command in python3 node systemctl install runuser curl; do
  command -v "$command" >/dev/null 2>&1 || { printf '%s is required\n' "$command" >&2; exit 1; }
done

create_service_user() {
  local user="$1"
  getent group "$user" >/dev/null 2>&1 || groupadd --system "$user"
  id "$user" >/dev/null 2>&1 || useradd --system --gid "$user" --home-dir /nonexistent --shell /usr/sbin/nologin "$user"
}

if [[ "${SKIP_BUILD:-false}" != "true" ]]; then
  for command in go npm; do
    command -v "$command" >/dev/null 2>&1 || { printf '%s is required unless SKIP_BUILD=true\n' "$command" >&2; exit 1; }
  done
  printf 'building Go Agent\n'
  (cd "$REPO_ROOT/agent-go" && go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/kylin-guard-agent ./cmd/server)
  printf 'building standalone web console\n'
  (cd "$REPO_ROOT/frontend" && npm ci && NEXT_TELEMETRY_DISABLED=1 COPILOTKIT_TELEMETRY_DISABLED=true KYLIN_GUARD_AGENT_API_URL=http://127.0.0.1:8080 npm run build)
fi

[[ -x "$REPO_ROOT/agent-go/bin/kylin-guard-agent" ]] || { printf 'agent binary is missing\n' >&2; exit 1; }
[[ -f "$REPO_ROOT/frontend/.next/standalone/server.js" ]] || { printf 'standalone web build is missing\n' >&2; exit 1; }

create_service_user kylinguard
create_service_user kylinguard-audit
create_service_user kylinguard-web

install -d -o root -g root -m 0755 "$INSTALL_ROOT/bin"
install -d -o kylinguard-audit -g kylinguard-audit -m 0750 "$INSTALL_ROOT/audit"
rm -rf "$INSTALL_ROOT/audit/app"
install -d -o kylinguard-audit -g kylinguard-audit -m 0750 "$INSTALL_ROOT/audit/app"
rm -rf "$INSTALL_ROOT/web"
install -d -o kylinguard-web -g kylinguard-web -m 0750 "$INSTALL_ROOT/web"
install -d -o root -g root -m 0755 "$CONFIG_ROOT"
install -o root -g root -m 0755 "$REPO_ROOT/agent-go/bin/kylin-guard-agent" "$INSTALL_ROOT/bin/kylin-guard-agent"

find "$REPO_ROOT/audit-core-py/app" -maxdepth 1 -type f -name '*.py' -exec install -o kylinguard-audit -g kylinguard-audit -m 0640 {} "$INSTALL_ROOT/audit/app/" \;
install -o kylinguard-audit -g kylinguard-audit -m 0640 "$REPO_ROOT/audit-core-py/requirements.txt" "$INSTALL_ROOT/audit/requirements.txt"
runuser -u kylinguard-audit -- python3 -m venv "$INSTALL_ROOT/audit/venv"
runuser -u kylinguard-audit -- "$INSTALL_ROOT/audit/venv/bin/python" -m pip install --disable-pip-version-check -r "$INSTALL_ROOT/audit/requirements.txt"

cp -a "$REPO_ROOT/frontend/.next/standalone/." "$INSTALL_ROOT/web/"
install -d -o kylinguard-web -g kylinguard-web -m 0750 "$INSTALL_ROOT/web/.next/static"
cp -a "$REPO_ROOT/frontend/.next/static/." "$INSTALL_ROOT/web/.next/static/"
if [[ -d "$REPO_ROOT/frontend/public" ]]; then
  cp -a "$REPO_ROOT/frontend/public" "$INSTALL_ROOT/web/public"
fi
chown -R kylinguard-web:kylinguard-web "$INSTALL_ROOT/web"

if [[ ! -e "$CONFIG_ROOT/agent.env" ]]; then
  install -o root -g kylinguard -m 0640 /dev/null "$CONFIG_ROOT/agent.env"
  printf '%s\n' \
    'KYLIN_GUARD_AGENT_ADDR=127.0.0.1:8080' \
    'AUDIT_CORE_URL=http://127.0.0.1:8001' \
    'EINO_RUNTIME_ENABLED=true' \
    'EINO_GRAPH_ENABLED=true' \
    'EINO_LLM_ENABLED=false' >"$CONFIG_ROOT/agent.env"
fi
if [[ ! -e "$CONFIG_ROOT/audit.env" ]]; then
  install -o root -g kylinguard-audit -m 0640 /dev/null "$CONFIG_ROOT/audit.env"
  printf 'TRACESHIELD_CORE_PATH=/opt/traceshield-core\n' >"$CONFIG_ROOT/audit.env"
fi

for unit in kylin-guard-agent.service kylin-guard-audit.service kylin-guard-web.service; do
  install -o root -g root -m 0644 "$SCRIPT_DIR/systemd/$unit" "/etc/systemd/system/$unit"
done
if command -v systemd-analyze >/dev/null 2>&1; then
  systemd-analyze verify \
    /etc/systemd/system/kylin-guard-agent.service \
    /etc/systemd/system/kylin-guard-audit.service \
    /etc/systemd/system/kylin-guard-web.service
fi
systemctl daemon-reload
systemctl enable --now kylin-guard-audit.service kylin-guard-agent.service kylin-guard-web.service
bash "$SCRIPT_DIR/check_stack.sh"
