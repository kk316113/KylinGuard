#!/usr/bin/env bash
set -euo pipefail

APP_HOME="${KYLINGUARD_HOME:-/opt/kylin-guard-agent}"
TRACE_HOME="${TRACESHIELD_CORE_PATH:-/opt/traceshield-core}"
AUDIT_PORT="${AUDIT_CORE_PORT:-8001}"
AGENT_PORT="${AGENT_GO_PORT:-8080}"
AUDIT_URL="${AUDIT_CORE_URL:-http://127.0.0.1:${AUDIT_PORT}}"
EINO_RUNTIME="${EINO_RUNTIME_ENABLED:-true}"
EINO_GRAPH="${EINO_GRAPH_ENABLED:-true}"
EINO_LLM="${EINO_LLM_ENABLED:-false}"

cd "$APP_HOME"

mkdir -p logs run

echo "== KylinGuard one-click start =="
echo "APP_HOME=$APP_HOME"
echo "TRACE_HOME=$TRACE_HOME"
echo "AUDIT_PORT=$AUDIT_PORT"
echo "AGENT_PORT=$AGENT_PORT"
echo "EINO_RUNTIME_ENABLED=$EINO_RUNTIME"
echo "EINO_GRAPH_ENABLED=$EINO_GRAPH"
echo "EINO_LLM_ENABLED=$EINO_LLM"
echo

if [ ! -d "$TRACE_HOME" ]; then
  echo "ERROR: TraceShield core path not found: $TRACE_HOME"
  exit 1
fi

echo "== stopping old services if any =="
bash scripts/linux/stop_all.sh || true

echo
echo "== starting audit-core-py =="
export TRACESHIELD_CORE_PATH="$TRACE_HOME"
export AUDIT_CORE_PORT="$AUDIT_PORT"

nohup bash deploy/kylin/run_audit_core_py.sh > logs/audit-core.log 2>&1 &
echo $! > run/audit-core.pid
echo "audit-core-py pid: $(cat run/audit-core.pid)"
echo "audit-core-py log: $APP_HOME/logs/audit-core.log"

echo
echo "== waiting for audit-core-py health =="
for i in $(seq 1 30); do
  if curl -s "http://127.0.0.1:${AUDIT_PORT}/health" >/tmp/kylin_guard_audit_health.json 2>/dev/null; then
    cat /tmp/kylin_guard_audit_health.json
    echo
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ERROR: audit-core-py did not become healthy"
    echo "---- audit-core log ----"
    tail -n 80 logs/audit-core.log || true
    exit 1
  fi
  sleep 1
done

echo
echo "== starting Go Agent =="
export AUDIT_CORE_URL="$AUDIT_URL"
export AGENT_GO_PORT="$AGENT_PORT"
export EINO_RUNTIME_ENABLED="$EINO_RUNTIME"
export EINO_GRAPH_ENABLED="$EINO_GRAPH"
export EINO_LLM_ENABLED="$EINO_LLM"

nohup bash deploy/kylin/run_agent_go.sh > logs/agent-go.log 2>&1 &
echo $! > run/agent-go.pid
echo "Go Agent pid: $(cat run/agent-go.pid)"
echo "Go Agent log: $APP_HOME/logs/agent-go.log"

echo
echo "== waiting for Go Agent health =="
for i in $(seq 1 30); do
  if curl -s "http://127.0.0.1:${AGENT_PORT}/health" >/tmp/kylin_guard_agent_health.json 2>/dev/null; then
    cat /tmp/kylin_guard_agent_health.json
    echo
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ERROR: Go Agent did not become healthy"
    echo "---- agent-go log ----"
    tail -n 80 logs/agent-go.log || true
    exit 1
  fi
  sleep 1
done

echo
echo "== running Linux/Kylin E2E test =="
bash scripts/linux/test_agent_e2e.sh | tee logs/e2e-latest.log

echo
echo "== all done =="
echo "audit-core-py: http://127.0.0.1:${AUDIT_PORT}/health"
echo "Go Agent:      http://127.0.0.1:${AGENT_PORT}/health"
echo "E2E log:       $APP_HOME/logs/e2e-latest.log"
echo
echo "To stop services:"
echo "  cd $APP_HOME && bash scripts/linux/stop_all.sh"
