# AGENTS.md

## Project Identity

KylinGuard / 麒盾 is a security-aware intelligent operations Agent for Kylin OS.

It is not a security audit dashboard. The main product line is:

```text
user natural-language ops task
-> LLM-driven Agent Loop
-> safe tool execution
-> observation
-> final_answer
```

TraceShield, intent_guard, Tool Policy, Exec Proxy, reasoning_trace, and risk graph are safety and explanation layers around tool calls.

## Product Baseline Docs

Use `AGENTS.md` as the compact workspace-memory entrypoint. Do not paste full product or API specs here.

- Product requirements baseline: `docs/product/PRODUCT_REQUIREMENTS_BASELINE.md`
- Agent API baseline: `docs/backend/AGENT_API_BASELINE.md`
- Current implementation state: `docs/agent_memory/CURRENT_STATE.md`
- Working rules and constraints: `docs/agent_memory/WORKING_RULES.md`

Development priority:

1. Agent experience first: user-facing `final_answer` is the main product output.
2. Guardrails are embedded in every tool call, not a separate after-the-fact dashboard.
3. Risk graph is an explanation layer generated from real execution evidence.
4. Natural-language tasks and prompt suggestions must not become fixed workflows.

## Current Baseline

- KylinGuard production Agent closure on Kylin V11 x86 VMware - PASS
- LLM-driven Agent Loop with safe tool execution - PASS
- Standard MCP endpoint with 13 advertised read-only tools - PASS
- Prompt injection / mutation guardrails - PASS
- Deep OS sensing tools - PASS
- RPM-backed configuration drift detector - PASS
- B/S frontend, Go Agent, and audit-core systemd stack - PASS
- Real DeepSeek multi-task Agent Loop acceptance - PASS
- LoongArch target status: linux/loong64 static build PASS; no VMware LoongArch runtime claim

Verified real DeepSeek summary:

```text
OPENAI_COMPATIBLE_BASE_URL=https://api.deepseek.com
OPENAI_COMPATIBLE_MODEL=deepseek-v4-flash
chat_model=remote-llm-deepseek-openai_compatible
chat_model contains mock: NO - PASS
agent_mode=agent_loop
natural-language acceptance tasks=4
passed=4
failed=0
final_answer=OK
fallback_reason=none
audit_result=OK
security_report=OK
```

## Runtime Modes

1. deterministic baseline
   - `chat_model=deterministic-stub`
   - fallback / regression / stable demo

2. mock LLM
   - `DEMO_MOCK_LLM=true`
   - `chat_model=remote-llm-mock-openai_compatible`
   - Agent Loop regression without real key

3. real DeepSeek
   - `DEMO_MOCK_LLM=false`
   - `OPENAI_COMPATIBLE_BASE_URL=https://api.deepseek.com`
   - `OPENAI_COMPATIBLE_MODEL=deepseek-v4-flash`
   - `OPENAI_COMPATIBLE_API_KEY` from environment only
   - `chat_model=remote-llm-deepseek-openai_compatible`

## Default Codex Workflow

When the user gives a normal development task:

1. Read this `AGENTS.md` first.
2. Do not scan the whole repository by default.
3. If extra context is needed, read `docs/agent_memory/CURRENT_STATE.md`.
4. If rules or constraints are unclear, read `docs/agent_memory/WORKING_RULES.md`.
5. Then read only the task-relevant source files.
6. Prefer targeted edits and small diffs.
7. Do not run broad grep/find unless the task cannot be solved with targeted reading.
8. Do not read unrelated frontend/backend files just to "understand the project".

## Non-Negotiable Rules

- Do not turn natural-language tasks into hardcoded scenarios.
- Do not write `if task contains SSH then ...` in Agent Loop logic.
- User task examples are acceptance samples, not fixed workflows.
- Do not use keyword rules as the main task router.
- Do not let `scene_type` decide tool order.
- mock LLM behavior is only a test double; never treat it as the real Agent.
- deterministic baseline is only fallback/regression, not the main Agent.
- LLM can only propose structured `next_action`; system decides whether/how to execute.
- Never let LLM execute raw shell directly.
- Never bypass Tool Policy.
- Never bypass Exec Proxy.
- Never fabricate tool results, audit reports, or risk graphs.
- Never commit real API keys.
- Never print real API keys in logs.
- `run/demo.env` must not store real keys; use `[REDACTED]`.

## Agent Loop Main Path

```text
User task
-> LLM outputs next_action
-> parse / schema validate
-> intent_guard / action safety check
-> Tool Policy
-> Exec Proxy
-> tool execution
-> observation
-> reasoning_trace / tool_trace
-> LLM outputs next_action again
-> final_answer
-> TraceShield audit
```

## Memory Self-Maintenance

After any completed stage, bug fix, or meaningful verification:

1. Update `docs/agent_memory/CURRENT_STATE.md` with:
   - what changed
   - new commit hash if any
   - verification result
   - next suggested work
2. Update `docs/agent_memory/WORKING_RULES.md` only if project rules changed.
3. Do not store:
   - real API keys
   - raw sensitive logs
   - large raw JSON responses
   - temporary `/tmp` file contents
4. Keep memory concise. Prefer summaries and key fields over long logs.

## Common Validation Commands

Backend:

```bash
go test ./...
```

Frontend:

```bash
npm run typecheck
npm run build
```

Scripts:

```bash
bash -n scripts/linux/start_demo.sh
bash -n scripts/linux/check_demo.sh
bash -n scripts/linux/stop_demo.sh
```

Do not claim PASS unless the command was actually run.

## Completion Report Format

For each task, report:

1. files changed
2. tests run and results
3. git diff summary
4. whether memory files were updated
5. whether real API keys are absent from diff
6. whether a commit is recommended
