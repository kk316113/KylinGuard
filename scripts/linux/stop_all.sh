#!/usr/bin/env bash
set -euo pipefail

APP_HOME="${KYLINGUARD_HOME:-/opt/kylin-guard-agent}"
cd "$APP_HOME"

echo "== stopping KylinGuard services =="

stop_pid_file() {
  local name="$1"
  local file="$2"

  if [ -f "$file" ]; then
    local pid
    pid="$(cat "$file" || true)"
    if [ -n "${pid:-}" ] && kill -0 "$pid" 2>/dev/null; then
      echo "stopping $name pid=$pid"
      kill "$pid" 2>/dev/null || true
      sleep 1
      if kill -0 "$pid" 2>/dev/null; then
        echo "force stopping $name pid=$pid"
        kill -9 "$pid" 2>/dev/null || true
      fi
    else
      echo "$name pid file exists but process is not running"
    fi
    rm -f "$file"
  else
    echo "$name pid file not found"
  fi
}

stop_pid_file "Go Agent" "run/agent-go.pid"
stop_pid_file "audit-core-py" "run/audit-core.pid"

echo "done"
