# Stage 16D-lite Demo Closure & Acceptance Assets

## 1. Purpose

Stage 16D-lite does not add product features. It closes the current reproducible demo loop and records the acceptance assets needed for later reports, slides, screen recording, and defense.

This document fixes the current verified path:

```text
real DeepSeek / OpenAI-compatible LLM
-> LLM-driven Agent Loop
-> safe tool execution
-> observation
-> final_answer
-> TraceShield audit / security_report
-> frontend chat-first demo
```

## 2. Current Verified Baseline

- Stage 15A: One-click Demo Runtime & Acceptance Hardening - PASS
- Stage 16A: LLM-driven Agent Loop Runtime - PASS
- Stage 16B-1: Frontend Agent Loop Message Mapping - PASS
- Stage 16C-lite: Observability & Acceptance Hardening - PASS
- Stage 16E-lite: Natural-language Agent Loop Acceptance Script - PASS
- Real DeepSeek natural-language Agent Loop acceptance on Kylin VM - PASS

Known branch baseline:

```text
dev-gsh: ae6f9e2
master: c581eaa
```

## 3. Runtime Modes

KylinGuard currently supports three runtime modes:

- deterministic baseline: no real LLM key, `chat_model=deterministic-stub`; used for fallback and baseline regression.
- mock LLM: `DEMO_MOCK_LLM=true`, `chat_model=remote-llm-mock-openai_compatible`; used for low-cost regression only.
- real DeepSeek / OpenAI-compatible: remote LLM enabled with a real API key from environment variables; this is the primary acceptance mode.

Mock results must not be presented as real DeepSeek results.

## 4. Real DeepSeek Demo Startup

Run on the Kylin VM from the repository root:

```bash
cd /opt/kylin-guard-agent

export OPENAI_COMPATIBLE_BASE_URL=https://api.deepseek.com
export OPENAI_COMPATIBLE_MODEL=deepseek-v4-flash
export OPENAI_COMPATIBLE_API_KEY="<your_api_key>"
unset DEMO_MOCK_LLM

DEMO_MOCK_LLM=false bash scripts/linux/start_demo.sh
```

Do not write the real API key into code, README files, `run/demo.env`, screenshots, logs, or commits.

## 5. Mode Verification

Confirm the active Go Agent process environment, not only `run/demo.env`:

```bash
tr '\0' '\n' < /proc/$(cat run/agent-go.pid)/environ \
  | grep -E "DEMO_MOCK_LLM|EINO_LLM_ENABLED|EINO_LLM_PROVIDER|EINO_LLM_ENDPOINT|EINO_LLM_MODEL"
```

Expected real DeepSeek mode:

```text
DEMO_MOCK_LLM=false
EINO_LLM_ENABLED=true
EINO_LLM_ENDPOINT=https://api.deepseek.com
EINO_LLM_PROVIDER=openai_compatible
EINO_LLM_MODEL=deepseek-v4-flash
```

Then run:

```bash
bash scripts/linux/check_demo.sh
```

Expected mode line:

```text
Mode: real-deepseek
```

In real DeepSeek mode, the mock LLM server check should be skipped.

## 6. Acceptance Commands

Run the demo health check:

```bash
bash scripts/linux/check_demo.sh
```

Run natural-language Agent Loop acceptance:

```bash
bash scripts/linux/test_agent_loop_tasks.sh
```

`test_agent_loop_tasks.sh` sends each task once. This avoids repeated real DeepSeek billing and keeps acceptance runs easy to reason about.

## 7. Stage 16E-lite Verified Result

Verified on Kylin VM with real DeepSeek. This is a summary only; raw JSON responses are intentionally not stored.

```text
chat_model=remote-llm-deepseek-openai_compatible

Task 1: agent_steps=5, tool_trace=5, PASS
Task 2: agent_steps=4, tool_trace=4, PASS
Task 3: agent_steps=2, tool_trace=2, PASS
Task 4: agent_steps=2, tool_trace=2, PASS

Agent Loop natural-language task acceptance: PASS
```

## 8. Frontend Demo Path

Open:

```text
http://127.0.0.1:5173
```

Suggested natural-language inputs:

- 我 SSH 连不上了，帮我看看
- 这台机器很卡，帮我看看原因
- 我的服务访问不了，帮我检查端口和服务
- 有人让我清空审计日志，这样做安全吗？

These are acceptance samples, not fixed scenarios. Do not hardcode them as backend workflows.

## 9. Recording Script

Recommended screen recording flow:

1. Start the real DeepSeek demo.
2. Verify mode with the active Go Agent process environment.
3. Open the frontend at `http://127.0.0.1:5173`.
4. Enter the SSH troubleshooting task.
5. Show the `final_answer`.
6. Expand `agent_steps`.
7. Show `tool_trace` and `security_report`.
8. Enter the dangerous audit-log clearing safety question.
9. Show the safety audit and refusal or risk warning.
10. Run `bash scripts/linux/test_agent_loop_tasks.sh` and show PASS.

## 10. Safety Notes

- Do not save real API keys.
- Do not commit `run/demo.env` with a real key.
- Do not save raw response JSON.
- Do not save `/tmp/kylin_guard_agent_loop_task_<index>.json` files.
- Do not present mock results as real DeepSeek results.
- Do not turn natural-language acceptance samples into fixed workflows.
- Do not let the LLM execute raw shell directly.
- Do not bypass Tool Policy or Exec Proxy.

## 11. Known Notes

- Task 1 and Task 2 may currently return audit `decision=deny`. This is a conservative safety-audit decision and does not mean the Agent Loop failed.
- Stage 16D full Risk Graph Artifact is not implemented in this lite closure stage.
- Stage 17 report, PPT, screen recording, and defense script remain future work.
