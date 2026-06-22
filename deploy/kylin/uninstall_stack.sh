#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  printf 'run this uninstaller as root\n' >&2
  exit 1
fi

for unit in kylin-guard-web.service kylin-guard-agent.service kylin-guard-audit.service; do
  systemctl disable --now "$unit" 2>/dev/null || true
  rm -f "/etc/systemd/system/$unit"
done
systemctl daemon-reload
rm -rf /opt/kylin-guard

for user in kylinguard-web kylinguard-audit kylinguard; do
  id "$user" >/dev/null 2>&1 && userdel "$user" || true
  getent group "$user" >/dev/null 2>&1 && groupdel "$user" || true
done

if [[ "${PURGE_CONFIG:-false}" == "true" ]]; then
  rm -rf /etc/kylin-guard
else
  printf 'configuration preserved at /etc/kylin-guard; set PURGE_CONFIG=true to remove it\n'
fi
printf 'KylinGuard services removed\n'
