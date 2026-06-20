#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
SOURCE_BINARY="${SOURCE_BINARY:-$REPO_ROOT/agent-go/bin/kylin-guard-agent}"
INSTALL_ROOT="/opt/kylin-guard"
CONFIG_ROOT="/etc/kylin-guard"
SERVICE_USER="kylinguard"
SERVICE_GROUP="kylinguard"
SERVICE_NAME="kylin-guard-agent.service"

if [[ "${EUID}" -ne 0 ]]; then
  printf 'run this installer as root\n' >&2
  exit 1
fi

if [[ ! -x "$SOURCE_BINARY" ]]; then
  printf 'built agent binary not found or not executable: %s\n' "$SOURCE_BINARY" >&2
  printf 'run deploy/kylin/install_agent_go.sh first, or set SOURCE_BINARY\n' >&2
  exit 1
fi

if ! getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
  groupadd --system "$SERVICE_GROUP"
fi

if ! id "$SERVICE_USER" >/dev/null 2>&1; then
  useradd \
    --system \
    --gid "$SERVICE_GROUP" \
    --home-dir /nonexistent \
    --shell /usr/sbin/nologin \
    --comment 'KylinGuard least-privilege service account' \
    "$SERVICE_USER"
fi

install -d -o root -g root -m 0755 "$INSTALL_ROOT/bin"
install -d -o root -g "$SERVICE_GROUP" -m 0750 "$CONFIG_ROOT"
install -o root -g root -m 0755 "$SOURCE_BINARY" "$INSTALL_ROOT/bin/kylin-guard-agent"
install -o root -g root -m 0644 \
  "$SCRIPT_DIR/systemd/$SERVICE_NAME" \
  "/etc/systemd/system/$SERVICE_NAME"

if [[ ! -e "$CONFIG_ROOT/agent.env" ]]; then
  install -o root -g "$SERVICE_GROUP" -m 0640 /dev/null "$CONFIG_ROOT/agent.env"
  {
    printf 'KYLIN_GUARD_AGENT_ADDR=127.0.0.1:8080\n'
    printf 'AUDIT_CORE_URL=http://127.0.0.1:8001\n'
    printf 'EINO_RUNTIME_ENABLED=true\n'
    printf 'EINO_GRAPH_ENABLED=true\n'
    printf 'EINO_LLM_ENABLED=false\n'
  } >"$CONFIG_ROOT/agent.env"
fi

systemctl daemon-reload
systemctl enable --now "$SERVICE_NAME"

printf 'installed %s under dedicated account %s\n' "$SERVICE_NAME" "$SERVICE_USER"
printf 'status: systemctl status %s\n' "$SERVICE_NAME"
printf 'logs:   journalctl -u %s\n' "$SERVICE_NAME"
printf 'health: curl -fsS http://127.0.0.1:8080/health\n'
