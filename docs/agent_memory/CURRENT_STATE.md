# CURRENT_STATE.md

## Current Status

KylinGuard has completed:

- Stage 15A: One-click Demo Runtime & Acceptance Hardening - PASS
- Stage 16A: LLM-driven Agent Loop Runtime - PASS
- Stage 16B-1: Frontend Agent Loop Message Mapping - PASS
- Stage 16C-lite: Observability & Acceptance Hardening - PASS
- Stage 16D-lite: Demo Closure & Acceptance Assets - PASS
- Stage 16E-lite: Natural-language Agent Loop Acceptance Script - PASS
- Stage 16F-lite: Frontend Demo Polish - PASS
- Stage 17A-UI-0: Frontend Framework / Template Reference Audit - PASS
- Stage 17A-BE-0: Product Shell Backend API Plan - PASS
- Stage 17A-1: Product Shell Implementation - IN PROGRESS
- Stage 17A-2: User-Facing Agent Experience Fix - IN PROGRESS
- Stage 17A-3: Semantic Interaction Router - IN PROGRESS
- Real DeepSeek Smoke Test - PASS
- Real DeepSeek natural-language acceptance on Kylin VM - PASS

## Latest Important Commits

- Stage 16B-1 frontend mapping: `d927a84`
- Stage 16C-lite observability/check_demo: master `355cb4e`
- chat_model metadata and key redaction:
  - dev-gsh: `52beac7`
  - master: `4976ed3`
- Stage 16E-lite acceptance script tightening: dev-gsh `79db46e`

## Real DeepSeek Verification

Verified on Kylin VM:

```text
DEMO_MOCK_LLM=false
EINO_LLM_ENABLED=true
EINO_LLM_ENDPOINT=https://api.deepseek.com
EINO_LLM_PROVIDER=openai_compatible
EINO_LLM_MODEL=deepseek-v4-flash

agent_mode: agent_loop
llm_enabled: True
remote_llm_used: True
chat_model: remote-llm-deepseek-openai_compatible
chat_model contains mock: NO - PASS
agent_steps: 3
tool_trace: 3
final_answer: OK
fallback_reason: none
audit_result: OK
security_report: OK
```

## Stage 16E-lite Natural-language Acceptance

Verified on Kylin VM with real DeepSeek:

```text
chat_model=remote-llm-deepseek-openai_compatible

Task 1: agent_steps=5, tool_trace=5, PASS
Task 2: agent_steps=4, tool_trace=4, PASS
Task 3: agent_steps=2, tool_trace=2, PASS
Task 4: agent_steps=2, tool_trace=2, PASS

Summary: passed=4, failed=0
Agent Loop natural-language task acceptance: PASS
```

## Stage 16D-lite Demo Closure

Demo closure and acceptance workflow have been documented in:

```text
docs/demo/STAGE_16D_LITE_DEMO_CLOSURE.md
```

The document records startup commands, mode verification, acceptance commands, frontend demo path, recording flow, safety notes, and known notes. It stores only summaries, not API keys, raw JSON, or `/tmp` response files.

## Stage 16F-lite Frontend Demo Polish

Frontend demo polish is complete:

- Assistant messages continue to prioritize `final_answer`.
- Agent Loop steps show clearer step cards with policy decision, reason or user summary, observation summary, and semantic fields when present.
- Runtime mode badge maps `chat_model` to Real DeepSeek Agent Loop, Mock Agent Loop, Deterministic Baseline, or generic Remote LLM Agent Loop.
- `decision=deny` is displayed as a safety audit state, not as a frontend request failure.

Verification:

```text
npm run typecheck - PASS
npm run build - PASS
```

## Stage 17A Scenario Workspace v1

Stage 17A implementation is pending. The previous direct Scenario Workspace implementation was discarded in favor of a planned implementation based on the UI reference plan and backend API plan.

Current accepted planning baseline:

- UI structure should follow `docs/frontend/UI_REFERENCE_AND_PRODUCT_SHELL_PLAN.md`.
- Backend product shell APIs should follow `docs/backend/PRODUCT_SHELL_BACKEND_API_PLAN.md`.
- Natural-language prompts remain inputs only, not scenario IDs.
- `scene_type` remains display and filtering metadata only, not workflow routing.

## Stage 17A-UI-0 Frontend Reference Audit

Frontend UI direction has been paused before further direct page development. The next product shell should be based on mature Agent Console / admin dashboard references rather than ad hoc generated UI.

Decision:

- Primary reference: Arco Design Pro Vue, because it matches Vue 3 + TypeScript + Arco Design Vue.
- Secondary reference: Vue Vben Admin for mature admin architecture only.
- Visual reference only: Art Design Pro.
- Product logic reference only: OpenClaw.

Plan document:

```text
docs/frontend/UI_REFERENCE_AND_PRODUCT_SHELL_PLAN.md
```

Hard rule: scene metadata and suggested prompts remain display/input aids only; they must not become fixed workflows or scenario IDs.

## Stage 17A-BE-0 Product Shell Backend API Plan

Backend implementation has been paused before adding more product shell endpoints. The backend API plan is documented in:

```text
docs/backend/PRODUCT_SHELL_BACKEND_API_PLAN.md
```

The plan defines read-only product shell APIs for runtime status, capabilities, and acceptance summary, plus the `run-eino` response contract for task session metadata:

- `GET /api/agent/runtime-status`
- `GET /api/agent/capabilities`
- `GET /api/agent/acceptance-summary`
- `POST /api/agent/run-eino` fields: `task_id`, `scene_type`, `scene_summary`, `run_status`, `created_at`

Rules recorded in the plan:

- status/capability/acceptance APIs must not call LLM;
- status/capability/acceptance APIs must not execute tools;
- no real API keys or raw response JSON should be returned or stored;
- `scene_type` is display and filtering metadata only, not tool routing.

## Stage 17A-1 Product Shell Implementation

Product Shell implementation is in progress:

- Backend implements read-only product shell APIs:
  - `GET /api/agent/runtime-status`
  - `GET /api/agent/capabilities`
  - `GET /api/agent/acceptance-summary`
- `run-eino` response supports task session fields:
  - `task_id`
  - `scene_type`
  - `scene_summary`
  - `run_status`
  - `created_at`
- Frontend implements an Agent Console shell:
  - Runtime Status Bar
  - Task Sidebar
  - Center Agent Workspace
  - Right Insight Panel with Steps / Evidence / Audit / Tools / Report tabs

Verification completed on Windows host:

```text
go test ./... - PASS
npm run typecheck - PASS
npm run build - PASS
API smoke for runtime-status / capabilities / acceptance-summary - PASS
```

Frontend-backend integration completed locally:

```text
Go backend started at 127.0.0.1:8080
Vite frontend started at 127.0.0.1:5173
Runtime Status Bar loaded runtime-status through the Vite proxy
Tools data loaded from capabilities through the Vite proxy
Acceptance summary loaded through the Vite proxy
Suggested prompt click sends natural-language text to /api/agent/run-eino
Task 1 local deterministic integration: decision=review, method=fallback-mock, scene_type=diagnosis, run_status=completed, plan_steps=2, tool_trace=2
Task 2 local dangerous intent integration: decision=deny, method=intent_guard, scene_type=security_check, run_status=blocked, tool_trace=0
Steps / Evidence / Audit / Report panels update after task response
Browser console critical warnings fixed for Product Shell integration
```

Pending verification:

```text
bash -n scripts/linux/check_demo.sh
bash -n scripts/linux/test_agent_loop_tasks.sh
Kylin VM demo smoke for Product Shell
```

The Windows host only exposes the WSL stub `bash.exe` and has no installed Linux distribution, so Linux shell syntax checks still need to be rerun on Kylin VM or another host with bash.

## Stage 17A-2 User-Facing Agent Experience Fix

Stage 17A-2 is in progress. The Product Shell UX has been refocused so the
assistant's user-facing answer is primary, while audit/security/evidence panels
remain secondary explanation surfaces.

Local work completed:

```text
Backend responses now include stable final_answer and user_message fields for deterministic, fallback, and intent_guard deny paths.
Dangerous intent responses now include a natural-language safety refusal while preserving decision=deny, run_status=blocked, method=intent_guard, and tool_trace=0.
Frontend center workspace now renders the assistant answer card first, followed by checked items, findings, and next steps.
Steps, Evidence, Audit, Tools, and Report remain in the right Insight Panel.
Loading copy shows task understanding, controlled checks, and answer preparation.
API errors are shown as user-readable task failures in the main workspace.
```

Pending:

```text
User manual browser confirmation
Kylin VM / real DeepSeek frontend-backend verification
Final PASS memory update
```

## Stage 17A-3 Semantic Interaction Router

Stage 17A-3 is in progress. The previous keyword-list routing idea was
discarded. The current implementation uses a semantic interaction router shape:
real LLM mode can classify chat / agent_run / safe_refusal / clarify via a
lightweight JSON-only router call, while deterministic mode uses conservative
fallback behavior and defaults ambiguous input to clarify instead of executing
tools.

Local behavior:

```text
normal chat: interaction_type=chat, agent_mode=chat_only, final_answer present, agent_steps=0, tool_trace=0, security_report=null
ambiguous input: interaction_type=clarify, final_answer asks for more details, agent_steps=0, tool_trace=0, security_report=null
ops task: interaction_type=agent_run, final_answer present, tool_trace/steps available as before
dangerous request: interaction_type=safe_refusal, router_source=safety_guard, decision=deny, run_status=blocked, tool_trace=0, natural-language safety refusal present
```

Pending:

```text
User manual browser confirmation
Kylin VM / real DeepSeek verification
Final PASS memory update
```

## CopilotKit Frontend Replacement

The old Vue/Vite frontend has been removed and a new Next.js + React +
TypeScript + CopilotKit frontend MVP has been created under `frontend/`.

Current implementation:

```text
Next.js App Router frontend runs on 127.0.0.1:5173
CopilotKit provider wraps the app as the Agent UX foundation
MVP uses existing non-streaming Go Agent APIs through Next rewrites
Agent Console shows final_answer first, then tool timeline and observations
Right Insight Panel shows Audit / Risk Graph / Hotspots / Decision Path / Tools / Report
Frontend does not store real API keys and does not decide tool execution
```

Verification on Windows host:

```text
npm run typecheck - PASS
npm run build - PASS
```

Pending:

```text
Manual browser review
Kylin VM / real DeepSeek frontend smoke
Future AG-UI streaming endpoint integration
```

## Product/API Baseline Docs

The product requirements and Agent API design baselines from the desktop docx
files have been converted into repository Markdown docs:

```text
docs/product/PRODUCT_REQUIREMENTS_BASELINE.md
docs/backend/AGENT_API_BASELINE.md
```

`AGENTS.md` now stays compact and points future Codex runs to these baseline
documents instead of embedding the full specifications. This is a documentation
baseline only and does not mark a new implementation stage as PASS.

The baseline docs record:

```text
Agent completion is the product mainline.
Guardrails are embedded into every tool call.
Risk graph is generated from real execution and audit evidence.
/api/agent/run is the target primary task API.
/api/agent/run-eino remains the current compatibility path.
```

## Current Next Suggested Work

Priority order:

1. Manually review the new CopilotKit frontend in browser
2. Rerun frontend smoke on Kylin VM with real DeepSeek
3. Decide whether to add AG-UI event streaming backend endpoint
4. Stage 17B: task history / report export planning
5. Stage 17: report / PPT / recording / defense script
6. Stage 18: packaging and final stability
