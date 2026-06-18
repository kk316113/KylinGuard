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

## Current Next Suggested Work

Priority order:

1. Stage 17A-BE-1: implement product shell read-only backend APIs
2. Stage 17A-FE-1: build frontend product shell from UI/API plans
3. Stage 17B: task history / report export planning
4. Stage 17: report / PPT / recording / defense script
5. Stage 18: packaging and final stability
