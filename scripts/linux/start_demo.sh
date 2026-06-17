#!/usr/bin/env bash
# Stage 15A: One-click KylinGuard Demo Runtime
# Starts audit-core-py, Go Agent, frontend, and optionally mock LLM.
#
# Usage:
#   bash scripts/linux/start_demo.sh                    # default deterministic demo
#   DEMO_MOCK_LLM=true bash scripts/linux/start_demo.sh  # mock LLM demo

set -euo pipefail

APP_HOME="${KYLINGUARD_HOME:-/opt/kylin-guard-agent}"
TRACE_HOME="${TRACESHIELD_CORE_PATH:-/opt/traceshield-core}"
AUDIT_PORT="${AUDIT_CORE_PORT:-8001}"
AGENT_PORT="${AGENT_GO_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"
MOCK_LLM_PORT="${MOCK_LLM_PORT:-8800}"
AUDIT_URL="${AUDIT_CORE_URL:-http://127.0.0.1:${AUDIT_PORT}}"
DEMO_MOCK_LLM="${DEMO_MOCK_LLM:-false}"

cd "$APP_HOME"
mkdir -p logs run

printf '\n========================================\n'
printf '  KylinGuard 麒盾 · Demo Runtime\n'
printf '========================================\n\n'
printf 'App home:      %s\n' "$APP_HOME"
printf 'TraceShield:   %s\n' "$TRACE_HOME"
printf 'Audit port:    %s\n' "$AUDIT_PORT"
printf 'Agent port:    %s\n' "$AGENT_PORT"
printf 'Frontend port: %s\n' "$FRONTEND_PORT"
printf 'Mock LLM:      %s\n' "$DEMO_MOCK_LLM"
printf '\n'

# --- Stop old services ---
printf '== Stopping old services ==\n'
bash "$APP_HOME/scripts/linux/stop_demo.sh" 2>/dev/null || true
printf '\n'

# --- Check TraceShield ---
if [ ! -d "$TRACE_HOME" ]; then
  printf 'WARNING: TraceShield core path not found: %s\n' "$TRACE_HOME"
  printf '  audit-core-py will run in fallback mode.\n'
fi

# --- Set LLM environment and start mock LLM if needed ---
printf '== Setting LLM mode ==\n'

# Detect real DeepSeek key (Stage 16A).
REAL_DEEPSEEK=false
if [ -n "${OPENAI_COMPATIBLE_API_KEY:-}" ]; then
  REAL_DEEPSEEK=true
elif [ -n "${OPENAI_API_KEY:-}" ]; then
  REAL_DEEPSEEK=true
  OPENAI_COMPATIBLE_API_KEY="$OPENAI_API_KEY"
fi

if [ "$REAL_DEEPSEEK" = "true" ]; then
  # Real DeepSeek mode — do NOT start mock server.
  export EINO_LLM_ENABLED=true
  export EINO_LLM_PROVIDER="${OPENAI_COMPATIBLE_PROVIDER:-openai_compatible}"
  export EINO_LLM_ENDPOINT="${OPENAI_COMPATIBLE_BASE_URL:-https://api.deepseek.com}"
  export EINO_LLM_MODEL="${OPENAI_COMPATIBLE_MODEL:-deepseek-v4-flash}"
  export EINO_LLM_API_KEY="$OPENAI_COMPATIBLE_API_KEY"
  printf '  LLM mode: real-deepseek\n'
  printf '  EINO_LLM_ENABLED=true\n'
  printf '  EINO_LLM_PROVIDER=%s\n' "$EINO_LLM_PROVIDER"
  printf '  EINO_LLM_ENDPOINT=%s\n' "$EINO_LLM_ENDPOINT"
  printf '  EINO_LLM_MODEL=%s\n' "$EINO_LLM_MODEL"
  printf '  EINO_LLM_API_KEY=[REDACTED]\n'
elif [ "$DEMO_MOCK_LLM" = "true" ]; then
  # Start mock LLM server before Go Agent so it's ready when agent starts.
  printf '  Starting mock LLM server on port %s...\n' "$MOCK_LLM_PORT"
  nohup python3 "$APP_HOME/scripts/dev/mock_openai_compatible_server.py" "$MOCK_LLM_PORT" \
    > "$APP_HOME/logs/mock-llm.log" 2>&1 &
  echo $! > "$APP_HOME/run/mock-llm.pid"
  printf '  Mock LLM pid: %s\n' "$(cat "$APP_HOME/run/mock-llm.pid")"

  for i in $(seq 1 15); do
    if curl -s -X POST "http://127.0.0.1:${MOCK_LLM_PORT}/v1/chat/completions" \
      -H "Content-Type: application/json" \
      -d '{"messages":[{"role":"user","content":"ping"}]}' >/dev/null 2>&1; then
      printf '  Mock LLM: OK\n'
      break
    fi
    if [ "$i" -eq 15 ]; then
      printf '  WARNING: mock LLM may not be ready, continuing...\n'
    fi
    sleep 1
  done

  export EINO_LLM_ENABLED=true
  export EINO_LLM_PROVIDER=openai_compatible
  export EINO_LLM_ENDPOINT="http://127.0.0.1:${MOCK_LLM_PORT}/v1/chat/completions"
  export EINO_LLM_MODEL=mock-model
  export EINO_LLM_API_KEY=sk-mock-key
  printf '  LLM mode: mock-openai-compatible\n'
  printf '  EINO_LLM_ENABLED=true\n'
  printf '  EINO_LLM_PROVIDER=openai_compatible\n'
  printf '  EINO_LLM_ENDPOINT=%s\n' "$EINO_LLM_ENDPOINT"
  printf '  EINO_LLM_API_KEY=[REDACTED]\n'
else
  export EINO_LLM_ENABLED=false
  export EINO_LLM_PROVIDER=deterministic
  unset EINO_LLM_ENDPOINT
  unset EINO_LLM_MODEL
  unset EINO_LLM_API_KEY
  printf '  LLM mode: deterministic\n'
  printf '  EINO_LLM_ENABLED=false\n'
fi
printf '\n'

# --- Save env for manual verification scripts ---
# Write demo.env with API key redacted for security.
cat > "$APP_HOME/run/demo.env" << EOF
export EINO_LLM_ENABLED=${EINO_LLM_ENABLED:-false}
export EINO_LLM_PROVIDER=${EINO_LLM_PROVIDER:-deterministic}
export EINO_LLM_ENDPOINT=${EINO_LLM_ENDPOINT:-}
export EINO_LLM_MODEL=${EINO_LLM_MODEL:-}
export EINO_LLM_API_KEY=${EINO_LLM_API_KEY:+[REDACTED]}
EOF
printf '  Saved env to run/demo.env\n'
printf '\n'

# --- Start audit-core-py ---
printf '== Starting audit-core-py ==\n'
export TRACESHIELD_CORE_PATH="$TRACE_HOME"
export AUDIT_CORE_PORT="$AUDIT_PORT"
nohup bash "$APP_HOME/deploy/kylin/run_audit_core_py.sh" > "$APP_HOME/logs/audit-core.log" 2>&1 &
echo $! > "$APP_HOME/run/audit-core.pid"
printf '  pid: %s\n' "$(cat "$APP_HOME/run/audit-core.pid")"

for i in $(seq 1 30); do
  if curl -s "http://127.0.0.1:${AUDIT_PORT}/health" >/dev/null 2>&1; then
    printf '  audit-core-py health: OK\n'
    break
  fi
  if [ "$i" -eq 30 ]; then
    printf '  ERROR: audit-core-py did not become healthy\n'
    tail -n 20 "$APP_HOME/logs/audit-core.log" 2>/dev/null || true
    exit 1
  fi
  sleep 1
done
printf '\n'

# --- Start Go Agent (env vars already set) ---
printf '== Starting Go Agent ==\n'
export AUDIT_CORE_URL="$AUDIT_URL"
export AGENT_GO_PORT="$AGENT_PORT"
export EINO_RUNTIME_ENABLED=true
export EINO_GRAPH_ENABLED=true

nohup bash "$APP_HOME/deploy/kylin/run_agent_go.sh" > "$APP_HOME/logs/agent-go.log" 2>&1 &
echo $! > "$APP_HOME/run/agent-go.pid"
printf '  pid: %s\n' "$(cat "$APP_HOME/run/agent-go.pid")"

for i in $(seq 1 30); do
  if curl -s "http://127.0.0.1:${AGENT_PORT}/health" >/dev/null 2>&1; then
    printf '  Go Agent health: OK\n'
    break
  fi
  if [ "$i" -eq 30 ]; then
    printf '  ERROR: Go Agent did not become healthy\n'
    tail -n 20 "$APP_HOME/logs/agent-go.log" 2>/dev/null || true
    exit 1
  fi
  sleep 1
done
printf '\n'

# --- Start frontend ---
printf '== Starting frontend ==\n'
NODE_CMD=""
if command -v node &>/dev/null; then
  NODE_CMD="node"
elif command -v nodejs &>/dev/null; then
  NODE_CMD="nodejs"
else
  printf '  ERROR: Node.js not found. Please install Node.js 18+ before starting frontend.\n'
  printf '  The backend is already running. You can start frontend manually:\n'
  printf '    cd %s/frontend && npm run dev\n' "$APP_HOME"
fi

if [ -n "$NODE_CMD" ]; then
  cd "$APP_HOME/frontend"
  nohup npm run dev -- --host 0.0.0.0 --port "$FRONTEND_PORT" \
    > "$APP_HOME/logs/frontend.log" 2>&1 &
  echo $! > "$APP_HOME/run/frontend.pid"
  printf '  pid: %s\n' "$(cat "$APP_HOME/run/frontend.pid")"
  sleep 3
  printf '  frontend: starting (check logs/frontend.log)\n'
  if curl -s "http://127.0.0.1:${FRONTEND_PORT}" >/dev/null 2>&1; then
    printf '  frontend: OK\n'
  else
    printf '  WARNING: frontend may not be ready yet; check logs/frontend.log\n'
  fi
fi
printf '\n'

# --- Demo URL ---
printf '========================================\n'
printf '  Demo ready!\n'
printf '  Frontend:  http://127.0.0.1:%s\n' "$FRONTEND_PORT"
printf '  Agent API: http://127.0.0.1:%s\n' "$AGENT_PORT"
printf '  Audit:     http://127.0.0.1:%s/health\n' "$AUDIT_PORT"
if [ "$DEMO_MOCK_LLM" = "true" ]; then
  printf '  Mock LLM:  http://127.0.0.1:%s/v1/chat/completions\n' "$MOCK_LLM_PORT"
fi
printf '\n'
printf '  Run health check:\n'
printf '    bash scripts/linux/check_demo.sh\n'
printf '\n'
printf '  Manual LLM verification:\n'
printf '    source run/demo.env\n'
printf '    bash scripts/linux/test_stage13b_remote_llm_manual.sh\n'
printf '\n'
printf '  Stop demo:\n'
printf '    bash scripts/linux/stop_demo.sh\n'
printf '\n'
printf '========================================\n'
