#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  printf 'run as root\n' >&2
  exit 1
fi
systemctl stop kylin-guard-web.service kylin-guard-agent.service kylin-guard-audit.service
printf 'KylinGuard stack stopped\n'
