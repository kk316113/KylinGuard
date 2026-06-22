#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  printf 'run as root\n' >&2
  exit 1
fi
systemctl start kylin-guard-audit.service kylin-guard-agent.service kylin-guard-web.service
bash "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/check_stack.sh"
