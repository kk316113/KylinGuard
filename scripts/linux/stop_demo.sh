#!/usr/bin/env bash
# Stage 15A: Stop all demo services.
# Stops: frontend, Go Agent, audit-core-py, mock LLM (if running).

set -euo pipefail

APP_HOME="${KYLINGUARD_HOME:-/opt/kylin-guard-agent}"
cd "$APP_HOME"

stop_pid() {
  local name="$1" pid_file="$2"
  if [ -f "$pid_file" ]; then
    local pid
    pid=$(cat "$pid_file" 2>/dev/null || echo "")
    if [ -n "$pid" ] && kill "$pid" 2>/dev/null; then
      printf '  Stopped %s (pid %d)\n' "$name" "$pid"
    else
      printf '  %s: already stopped\n' "$name"
    fi
    rm -f "$pid_file"
  else
    printf '  %s: not running\n' "$name"
  fi
}

printf '== Stopping demo services ==\n'
stop_pid frontend "$APP_HOME/run/frontend.pid"
stop_pid agent-go "$APP_HOME/run/agent-go.pid"
stop_pid audit-core "$APP_HOME/run/audit-core.pid"
stop_pid mock-llm "$APP_HOME/run/mock-llm.pid"
printf 'Done.\n'
