# CURRENT_STATE.md

## Current Status

KylinGuard production Agent closure is complete for the current competition
product boundary:

- Main API path `/api/agent/run` and compatibility path `/api/agent/run-eino` - PASS
- LLM-driven Agent Loop with structured `next_action` and safe tool execution - PASS
- Tool Policy and Exec Proxy enforcement on every tool call - PASS
- Prompt injection, unauthorized mutation, unknown action, and sensitive path guardrails - PASS
- TraceShield audit integration with trace-backed local safety fallback - PASS
- Standard MCP Streamable HTTP endpoint - PASS
- 13 MCP/Agent read-only tools, including `configuration_drift_detector` - PASS
- RPM-backed configuration drift detection through strict `rpm --verify <package>` arguments - PASS
- Deep OS sensing tools for lsof/process/disk I/O - PASS
- Next.js B/S frontend and Go Agent / audit-core / web systemd stack - PASS
- Kylin V11 x86_64 VMware full-stack runtime acceptance - PASS
- Real DeepSeek natural-language multi-task Agent Loop acceptance - PASS
- linux/loong64 static build - PASS

LoongArch note: the project has a successful `GOOS=linux GOARCH=loong64`
static build. No LoongArch VMware/runtime execution is claimed without a real
LoongArch Kylin V11 environment.

## Latest Important Commits

- Stage 16B-1 frontend mapping: `d927a84`
- Stage 16C-lite observability/check_demo: master `355cb4e`
- chat_model metadata and key redaction:
  - dev-gsh: `52beac7`
  - master: `4976ed3`
- Stage 16E-lite acceptance script tightening: dev-gsh `79db46e`
- Production B/S and Kylin systemd deployment closure: `d3dc6ff`
- Eino/Sonic runtime compatibility under systemd sandbox: `c83b94f`
- Kylin lsof acceptance under service isolation: `5771893`
- Remote LLM environment sanitization: `8cffe06`
- Remote LLM timeout and robust Agent Loop JSON extraction: `5f030d2`

## Final Kylin V11 VMware Acceptance

Verified on Kylin Linux Advanced Server V11 x86_64 running in VMware.

Installed services:

```text
kylin-guard-agent.service - active
kylin-guard-audit.service - active
kylin-guard-web.service - active
```

Runtime checks:

```text
Agent health: PASS
Audit health: PASS
Web health: PASS
Non-root service accounts: PASS
Systemd stack install/check scripts: PASS
B/S browser flow: PASS
MCP initialize / tools/list / tools/call: PASS
MCP tools/list count: 13
safe_shell not advertised: PASS
configuration_drift_detector acceptance: PASS
security guardrail attacks: PASS
deep OS sensing tools: PASS
```

Real DeepSeek Agent Loop acceptance:

```text
OPENAI_COMPATIBLE_BASE_URL=https://api.deepseek.com
OPENAI_COMPATIBLE_MODEL=deepseek-v4-flash
chat_model=remote-llm-deepseek-openai_compatible

Task 1 SSH diagnosis: PASS
Task 2 performance diagnosis: PASS
Task 3 service/port diagnosis: PASS
Task 4 dangerous audit-log clearing request: PASS

Summary: passed=4, failed=0
Agent Loop natural-language task acceptance: PASS
fallback_reason=none for diagnostic real-LLM tasks
```

Important implementation notes:

```text
Remote LLM fallback state is request-scoped.
Remote LLM HTTP responses are size-limited.
Remote LLM calls use bounded timeout/retry and honor context cancellation.
Agent Loop parser accepts a single extracted JSON object from otherwise noisy model output, then still schema-validates it.
Unknown action_type returns a controlled diagnostic observation and does not execute tools.
Audit service outage uses trace-backed local safety fallback, not fabricated TraceShield results.
No real API keys are stored in repository files or memory docs.
```

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
Next.js frontend started at 127.0.0.1:5173
Runtime Status Bar loaded runtime-status through Next rewrites
Tools data loaded from capabilities through Next rewrites
Acceptance summary loaded through Next rewrites
CopilotSidebar sends free-form messages through CopilotKit Runtime to the Agent API
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
CopilotKit Runtime adapts the existing Go Agent API to AG-UI events
CopilotKit's native CopilotSidebar owns the chat drawer, messages, and input
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
/api/agent/run is the primary task API.
/api/agent/run-eino remains a compatibility path for older scripts.
```

## Agent API Baseline Convergence MVP

Implementation is in progress. The backend now converges the product baseline
around `/api/agent/run` while preserving `/api/agent/run-eino` as a compatibility
alias.

Current local implementation:

```text
POST /api/agent/run uses the Agent Loop adapter as the primary task API.
POST /api/agent/run-eino remains available for existing acceptance scripts.
Each run receives run_id / task_id / scene_type / scene_summary / run_status / created_at.
An in-memory recent run store keeps the latest responses without adding a database.
GET /api/agent/runs/{run_id} returns the stored run response.
GET /api/agent/runs/{run_id}/audit-reports returns per-step audit reports when present.
GET /api/agent/runs/{run_id}/risk-graph returns the real backend risk_graph or an empty graph.
GET /api/agent/runs/{run_id}/report returns a lightweight report summary.
Frontend runAgentTask now calls /api/agent/run.
Frontend visible Chinese copy has been normalized for browser review.
```

Verification so far:

```text
go test ./... - PASS
npm run typecheck - PASS
npm run build - PASS
browser integration - pending
```

## CopilotKit Native Sidebar Integration

Current frontend interaction contract:

```text
Main canvas: left navigation + single-column operations/security dashboard.
Chat surface: CopilotKit v2's native expandable CopilotSidebar from @copilotkit/react-core/v2.
The frontend does not replace CopilotKit Messages or Input components.
The legacy @copilotkit/react-ui package and the custom chat drawer have been removed.
The application imports CopilotKit's official v2 stylesheet without overriding its drawer selectors.
CopilotKit Runtime v2 forwards free-form messages to the existing Go Agent API.
An AG-UI frontend tool synchronizes the completed run to the dashboard by run_id.
Audit, evidence, risk graph, tools, runs, and settings are board views.
decision=deny is displayed as a security state, not a request failure.
No backend Agent Loop logic was changed.
No fixed workflow or scenario routing was added.
Static prompt chips and leading task examples were removed from the frontend.
The application font stack and visual tokens now align with CopilotKit defaults.
```

Verification:

```text
npm run typecheck - PASS
npm run build - PASS
Local dev page returns HTTP 200 at 127.0.0.1:5173.
CopilotKit /info and /threads endpoints return HTTP 200.
AG-UI run smoke returned a non-empty final answer and completed run events.
Headless Edge verified the native v2 trigger and 480px expanded sidebar.
CopilotKit Inspector and development controls are explicitly disabled.
Native chat submission renders user/assistant messages and updates the dashboard.
The internal dashboard synchronization tool has no user-visible tool card.
Frontend source no longer contains rejected fixed prompt examples.
```

## CopilotKit Chat Backend MVP

The minimal chat backend path is runnable:

```text
CopilotKit v2 CopilotSidebar
-> POST /api/copilotkit/agent/default/run (SSE / AG-UI)
-> POST /api/agent/run (Go)
-> semantic interaction router
-> remote ChatModel answer with bounded conversation history when configured
-> deterministic greeting fallback otherwise
```

Chat-only responses do not execute tools, do not call the audit client, and return
empty `agent_steps` and `tool_trace`. The Go handler rejects empty tasks with HTTP
400 and limits request bodies. The CopilotKit adapter applies a backend timeout and
returns a readable connection error when the Go service is unavailable.

The deterministic fallback no longer uses a chat whitelist. Non-operational input
defaults to safe chat, while explicit operations requests still enter the Agent Loop.
Runtime/model identity questions report the actual configured `chat_model`; when no
remote key is configured, the answer explicitly identifies `deterministic-stub`.
Windows startup now accepts `OPENAI_COMPATIBLE_*`, `OPENAI_API_KEY`, or
`DEEPSEEK_API_KEY` aliases directly; explicit `EINO_LLM_*` values keep precedence.

Verification on the Windows development host:

```text
go test ./... - PASS
npm run typecheck - PASS
npm run build - PASS
POST /api/agent/run with a greeting - PASS
CopilotKit AG-UI SSE run with a greeting - PASS
Real DeepSeek CopilotKit chat smoke - PASS
chat_model=remote-llm-deepseek-openai_compatible
configured model identifier=deepseek-v4-flash
No real API key stored or printed
```

## Stage 18A Competition A2 Standard MCP Server

The official 2026 China Software Cup A2 requirements have been mapped in:

```text
docs/product/COMPETITION_A2_DELIVERY_PLAN.md
```

Local implementation now provides an official MCP Go SDK Streamable HTTP endpoint at `/mcp`:

```text
MCP initialize / tools/list / tools/call
-> registered direct-call tools only
-> JSON Schema validation
-> Tool Policy
-> existing Tool Registry / Exec Proxy
-> tool trace
-> TraceShield audit client
```

`safe_shell` and other non-direct-call tools are not advertised. Policy-denied MCP calls produce a denied trace and do not invoke the tool handler.

Verification completed on the Windows development host:

```text
go test ./... - PASS
official MCP in-memory client list/call tests - PASS
official MCP Streamable HTTP negotiation/list test - PASS
prompt-injected process_inspector call denied before handler execution - PASS
CGO_ENABLED=0 GOOS=linux GOARCH=loong64 go build ./cmd/server - PASS
```

Pending before Stage 18A PASS:

```text
Run the loong64 binary on the provided Kylin Advanced Server OS V11 VM
Verify MCP initialize / tools/list / tools/call on that VM
Verify real OS observations and audit-core integration on that VM
```

Kylin Advanced Server V11 x86_64 runtime verification completed on 2026-06-21:

```text
OS: Kylin Linux Advanced Server V11 (Swan25)
architecture: x86_64
runtime account: lihuan (uid=1000, no sudo used)
MCP initialize: PASS, protocolVersion=2025-06-18
MCP tools/list: PASS, 12 tools, safe_shell absent
MCP allowed disk_io_checker call through Exec Proxy: PASS
MCP sensitive-path denial: PASS, isError=true, decision=deny, method=tool_policy
```

The runtime check exposed and fixed an MCP audit precedence defect: a Tool Policy denial could retain the fallback auditor's `review` decision. Policy denial is now authoritative and includes tool-policy violation/evidence fields. Full Go tests and Kylin runtime retest pass.

The first least-privilege deployment scaffold is also present:

```text
deploy/kylin/install_agent_service.sh
deploy/kylin/systemd/kylin-guard-agent.service
```

It installs the backend under a dedicated `kylinguard` system account, binds to loopback by default, removes Linux capabilities, forbids privilege escalation, and enables systemd filesystem/device/kernel hardening. Git Bash `bash -n` passes locally; the unit still requires `systemd-analyze verify` and runtime tool checks on Kylin V11 before it can be marked PASS.

## Stage 18B Prompt Injection and Mutation Guardrails

The first competition-focused anti-injection hardening pass is implemented:

```text
direct prompt injection -> Intent Guard deny with threat_type=prompt_injection
prior chat / tool-observation injection -> neutralized in model-facing context
original observation -> retained in audit trace
unknown tool arguments -> Tool Policy deny
nested shell metacharacters -> Tool Policy deny
safe_shell -> absent from LLM and MCP tool definitions
systemctl -> explicit read-only actions only
cat -> /etc/os-release and selected /proc facts only
```

Verification completed locally:

```text
go test ./... - PASS
direct prompt injection produces decision=deny, run_status=blocked, tool_trace=0 - PASS
indirect observation/history injection neutralization tests - PASS
systemctl mutation and sensitive cat path denial tests - PASS
CGO_ENABLED=0 GOOS=linux GOARCH=loong64 go build ./cmd/server - PASS
bash -n scripts/linux/test_security_guardrails.sh - PASS
local HTTP guardrail acceptance (4 attack classes) - PASS
```

Kylin V11 x86_64 runtime execution is now PASS for all four guardrail attack classes. LoongArch repetition remains required before final competition acceptance.

## Stage 18C A2 Deep OS Sensing Tools

Three general-purpose read-only sensing capabilities have been added to the shared Tool Registry, so they are available to both the LLM Agent Loop and the standard MCP server without fixed scenario routing:

```text
open_file_inspector
  -> bounded lsof field output
  -> approved operational path or numeric PID only
  -> file contents are never read

process_inspector
  -> ALL / RUNNING / SLEEPING / ZOMBIE / STOPPED filters
  -> complete zombie count even when displayed records are limited
  -> low / medium / high zombie accumulation risk

disk_io_checker
  -> bounded two-sample /proc/diskstats reader
  -> IOPS, bytes/sec, utilization, in-progress I/O, weighted I/O time
  -> physical whole-disk filtering for sd/vd/xvd/hd/nvme/mmcblk names
```

Security constraints:

```text
lsof command arguments are independently enforced by Exec Proxy
/etc, /root, /home, /proc and relative path inspection are denied
sample interval is bounded to 100-2000 ms
safe_shell remains absent from Agent and MCP tool definitions
```

Verification completed locally:

```text
go test ./... - PASS
MCP default tool discovery for all three sensing tools - PASS
lsof parser, path policy and Exec Proxy argument tests - PASS
ps zombie parser/filter/risk tests - PASS
diskstats parser/delta/risk tests - PASS
CGO_ENABLED=0 GOOS=linux GOARCH=loong64 go build ./cmd/server - PASS
bash -n scripts/linux/test_os_sensing_tools.sh - PASS
```

The Windows host has no Docker or Linux runtime, so runtime verification was executed over SSH on the provided Kylin V11 VM with `scripts/linux/test_os_sensing_tools.sh`.

Kylin V11 x86_64 runtime execution is now PASS:

```text
real lsof holder PID detection - PASS
zombie process sensing - PASS
live /proc/diskstats sampling (sda) - PASS
sensitive lsof path denial - PASS
process identity - uid=1000 lihuan, no shell/sudo execution in tool traces
temporary validation service stopped - PASS
```

The deterministic no-key Agent Loop also completed a natural-language performance task with `agent_mode=agent_loop`, one real `os_info` trace, per-step aggregate audit, and a non-empty final answer. This is fallback-path evidence only; it does not replace the existing or future real DeepSeek multi-step acceptance.

## Current Next Suggested Work

Priority order:

1. Rotate any real DeepSeek API key that was exposed outside the VM environment.
2. If a real LoongArch Kylin V11 machine becomes available, repeat install, MCP, guardrail, configuration drift, Agent Loop, and B/S acceptance there.
3. Prepare final competition submission artifacts and the demo script from the verified production runtime.
4. Optional hardening: add repeatable API/Agent latency benchmarks and release packaging metadata.
