# Product Shell Backend API Plan

> Current status: this Stage 17A planning document has been superseded by the
> implemented production shell. `/api/agent/run` is now the primary Agent Loop
> task API, and `/api/agent/run-eino` remains a compatibility path.

## 1. Purpose

Stage 17A-BE-0 does not implement new backend features. It defines the minimum backend API shape required by the Stage 17 product shell so the frontend can become an Agent Console instead of a raw JSON viewer or a security-audit dashboard.

The goal is to support this product line:

```text
user natural-language ops task
-> LLM-driven Agent Loop
-> safe tool execution
-> observations and evidence
-> final_answer
-> security explanation
-> product shell display
```

The frontend needs stable, read-only status and capability APIs around the existing Agent Loop. These APIs must not trigger LLM calls, must not execute tools, and must not expose secrets.

## 2. Current Backend Baseline

Current backend capabilities:

- `GET /health` returns Go Agent health.
- `GET /api/os/info` invokes the `os_info` tool.
- `POST /api/agent/run` runs the Agent Loop and is the primary task API.
- `POST /api/agent/run-eino` calls the same Agent Loop runtime as a compatibility endpoint.
- `GET /api/agent/runs/{run_id}`, `/audit-reports`, `/risk-graph`, and `/report` expose the latest in-memory run artifacts.
- `GET /api/tools`, `GET /api/tools/{name}`, and `POST /api/tools/call` expose the MCP-like tool protocol and guarded tool execution path.
- `POST /mcp` exposes the standard MCP Streamable HTTP tool endpoint.

Current Agent Loop safety path:

```text
task
-> intent_guard
-> LLM next_action
-> schema validation
-> Tool Policy
-> Exec Proxy / Registry invocation
-> observation
-> tool_trace
-> audit-core / TraceShield
-> security_report
-> final_answer
```

Current response data already useful for the product shell:

- `agent_mode`
- `task_understanding`
- `agent_steps`
- `final_answer`
- `tool_trace`
- `audit_result`
- `security_report`
- `reasoning_trace`
- runtime metadata under `security_report.audit_metadata`

Stage 17A direct implementation work has also introduced response-level task session metadata in the working tree, but this document treats that as an implementation target, not a dependency for the API plan.

## 3. Frontend Product Shell Needs

The product shell should display an operations task session, not a raw audit report. It needs a small set of stable contracts:

| UI Area | Backend Need | Proposed Source |
|---|---|---|
| Top Status Bar | Agent health, audit-core URL reachability, runtime mode, model display, policy status | `GET /api/agent/runtime-status` |
| Left Sidebar | Available operations capabilities and safe prompt categories | `GET /api/agent/capabilities` |
| Center Agent Workspace | Natural-language task run, final answer, execution steps | `POST /api/agent/run` |
| Right Insight Panel | tool evidence, security report, audit result, reasoning spans | `POST /api/agent/run` response and run artifact APIs |
| Demo Acceptance Panel | verified stages and safe commands to rerun | `GET /api/agent/acceptance-summary` |

The API layer should avoid forcing the frontend to infer runtime mode by digging through several nested fields. It should also avoid requiring the frontend to know whether a result came from mock mode, deterministic baseline, or real DeepSeek by parsing logs or shell environment files.

## 4. Proposed APIs

### 4.1 `GET /api/agent/runtime-status`

Purpose: provide a read-only runtime snapshot for the top status bar.

Rules:

- Must not call the LLM.
- Must not execute diagnosis tools.
- Must not print or return API keys.
- May inspect loaded config and lightweight service health only.
- If audit-core reachability is checked, use a short timeout and only call `/health`.

Example response:

```json
{
  "status": "ok",
  "service": "kylin-guard-agent",
  "version": "0.1.0",
  "runtime": {
    "route": "eino_graph_runtime",
    "agent_mode": "agent_loop",
    "eino_runtime_enabled": true,
    "eino_graph_enabled": true,
    "llm_enabled": true,
    "remote_llm_used": true,
    "provider": "openai_compatible",
    "endpoint": "https://api.deepseek.com",
    "model": "deepseek-v4-flash",
    "chat_model": "remote-llm-deepseek-openai_compatible",
    "mode_label": "Real DeepSeek Agent Loop"
  },
  "services": {
    "go_agent": {
      "status": "ok"
    },
    "audit_core": {
      "status": "ok",
      "url": "http://127.0.0.1:8001",
      "traceshield_available": true
    }
  },
  "security_layers": {
    "intent_guard": "enabled",
    "tool_policy": "enabled",
    "exec_proxy": "enabled",
    "traceshield_audit": "enabled"
  },
  "secret_safety": {
    "api_key_configured": true,
    "api_key_redacted": true
  },
  "updated_at": "2026-06-18T10:00:00Z"
}
```

Notes:

- `api_key_configured` is allowed as a boolean.
- The API must never return the key value, key prefix, or key suffix.
- `mode_label` is a presentation helper only; it must not affect runtime behavior.

### 4.2 `GET /api/agent/capabilities`

Purpose: give the frontend a safe capabilities model for sidebars, tool panels, and help text.

Rules:

- Must not execute tools.
- Must not call LLM.
- Should derive tool metadata from the existing registry and policy metadata.
- Should avoid exposing raw command templates if those would encourage unsafe manual execution.

Example response:

```json
{
  "agent": {
    "supports_natural_language_tasks": true,
    "supports_agent_loop": true,
    "supports_remote_llm": true,
    "supports_mock_llm": true,
    "supports_deterministic_baseline": true
  },
  "tools": [
    {
      "name": "service_status",
      "description": "Check service status",
      "enabled": true,
      "operation_type": "read",
      "resource_type": "service",
      "boundary_level": "low",
      "requires_privilege": false,
      "policy_summary": "read-only service inspection"
    }
  ],
  "tool_policy": {
    "default_deny_for_unknown_tools": true,
    "dangerous_shell_blocked": true,
    "raw_shell_execution": "not_allowed"
  },
  "audit": {
    "audit_core_url_configured": true,
    "traceshield_adapter": "http"
  }
}
```

### 4.3 `GET /api/agent/acceptance-summary`

Purpose: provide the product shell and demo panel with a concise, non-sensitive acceptance baseline.

Rules:

- Must not run acceptance scripts.
- Must not call LLM.
- Must not read `/tmp` response files.
- Must not store or return raw JSON.
- Should return documented verification summaries only.

Example response:

```json
{
  "baseline": {
    "stage_15a": "PASS",
    "stage_16a": "PASS",
    "stage_16b_1": "PASS",
    "stage_16c_lite": "PASS",
    "stage_16d_lite": "PASS",
    "stage_16e_lite": "PASS",
    "stage_16f_lite": "PASS"
  },
  "real_deepseek_acceptance": {
    "status": "PASS",
    "chat_model": "remote-llm-deepseek-openai_compatible",
    "tasks": [
      {
        "index": 1,
        "agent_steps": 5,
        "tool_trace": 5,
        "result": "PASS"
      },
      {
        "index": 2,
        "agent_steps": 4,
        "tool_trace": 4,
        "result": "PASS"
      },
      {
        "index": 3,
        "agent_steps": 2,
        "tool_trace": 2,
        "result": "PASS"
      },
      {
        "index": 4,
        "agent_steps": 2,
        "tool_trace": 2,
        "result": "PASS"
      }
    ]
  },
  "commands": {
    "check_demo": "bash scripts/linux/check_demo.sh",
    "agent_loop_acceptance": "bash scripts/linux/test_agent_loop_tasks.sh"
  },
  "notes": [
    "Acceptance samples are natural-language examples, not fixed workflows.",
    "Mock mode is regression-only and must not be presented as real DeepSeek."
  ]
}
```

### 4.4 Agent Run Response Contract

Purpose: preserve the Agent Loop response shape for both the primary
`POST /api/agent/run` endpoint and the compatible `POST /api/agent/run-eino`
endpoint.

Existing fields to preserve:

- `task`
- `decision`
- `summary`
- `agent_mode`
- `task_understanding`
- `agent_steps`
- `final_answer`
- `tool_trace`
- `audit_result`
- `security_report`
- `reasoning_trace`

Stage 17A session fields to stabilize:

```json
{
  "task_id": "kg-20260618-abc123",
  "scene_type": "diagnosis",
  "scene_summary": "SSH connectivity diagnosis",
  "run_status": "completed",
  "created_at": "2026-06-18T10:00:00Z"
}
```

Field rules:

- `task_id` is generated per request. No database is required in Stage 17A.
- `scene_type` is display and filtering metadata only. It must not determine tool order.
- `scene_summary` should prefer existing task-understanding output where available, otherwise use a short display summary.
- `run_status` should be one of `completed`, `blocked`, `failed`, or `partial`.
- `created_at` should be an ISO timestamp.

Allowed `scene_type` values:

- `diagnosis`
- `security_check`
- `service_recovery`
- `system_health`
- `compliance_review`
- `unknown`

### 4.5 Future Task Run APIs

These are not required for Stage 17A.

Potential Stage 17B APIs:

- `GET /api/agent/runs`
- `GET /api/agent/runs/{task_id}`
- `GET /api/agent/runs/{task_id}/report.md`
- `POST /api/agent/runs/{task_id}/export`

These require a persistence or artifact strategy and should not be squeezed into Stage 17A.

## 5. Security Rules

Backend product shell APIs must follow these rules:

- Never return real API keys.
- Never log real API keys.
- Never trigger a real LLM call from status, capabilities, or acceptance summary APIs.
- Never execute tools from status, capabilities, or acceptance summary APIs.
- Never bypass `intent_guard`.
- Never bypass Tool Policy.
- Never bypass Exec Proxy.
- Never expose raw shell execution as a frontend feature.
- Never turn natural-language examples into fixed workflows.
- Never let `scene_type` determine tool routing or tool order.
- Never present mock mode as real DeepSeek mode.

## 6. Implementation Plan

### Stage 17A-BE-1

Implement the minimum product shell read APIs:

- `GET /api/agent/runtime-status`
- `GET /api/agent/capabilities`
- `GET /api/agent/acceptance-summary`

Also stabilize the `POST /api/agent/run-eino` response fields:

- `task_id`
- `scene_type`
- `scene_summary`
- `run_status`
- `created_at`

This is the backend scope required before the product shell frontend should consume status and capability data.

### Stage 17A-FE-1

Build the frontend product shell against the above contracts:

- top runtime status bar
- left task/capability sidebar
- center chat-first Agent workspace
- right insight panel
- compact acceptance/demo panel

The frontend should treat `decision=deny` as a safety state, not a request failure.

### Stage 17B

Defer persistence and report export:

- task history
- task replay
- exported Markdown or PDF reports
- long-term run artifacts
- full Risk Graph artifact display

## 7. Suggested File Changes for Implementation

Likely backend files:

- `agent-go/cmd/server/main.go`
  - register new read-only handlers.
- `agent-go/internal/config/config.go`
  - expose sanitized runtime configuration helpers if needed.
- `agent-go/internal/eino/runtime.go`
  - keep runtime metadata consistent with the Agent Loop response.
- `agent-go/internal/agent/runtime.go`
  - keep stable runtime response compatibility.
- `agent-go/internal/tools/registry.go`
  - reuse tool metadata for capabilities.

Likely frontend files:

- `frontend/src/api/agent.ts`
  - add typed clients for the new read-only APIs.
- `frontend/src/types/agent.ts`
  - add product shell API response types.
- `frontend/src/pages/AgentChatWorkbench.vue`
  - consume runtime status and capability APIs after product shell layout is approved.

Scripts to keep compatible:

- `scripts/linux/check_demo.sh`
- `scripts/linux/test_agent_loop_tasks.sh`

## 8. Acceptance Criteria

Backend acceptance:

- `go test ./...` passes after implementation.
- `GET /api/agent/runtime-status` returns sanitized runtime status and no secrets.
- `GET /api/agent/capabilities` returns tool metadata without executing tools.
- `GET /api/agent/acceptance-summary` returns documented acceptance summaries without running LLM or scripts.
- `POST /api/agent/run-eino` still runs the Agent Loop and returns final answer, steps, traces, security report, and audit result.
- Stage 17A session fields are present in `run-eino` responses.

Frontend acceptance:

- `npm run typecheck` passes.
- `npm run build` passes.
- Product shell can render runtime status, capabilities, task session metadata, final answer, steps, evidence, and security explanation.
- `decision=deny` is rendered as a safety audit state, not as a frontend request failure.

Script acceptance:

- `bash -n scripts/linux/check_demo.sh` passes.
- `bash -n scripts/linux/test_agent_loop_tasks.sh` passes.
- Real DeepSeek acceptance remains a one-request-per-task workflow.

Security acceptance:

- No real API key appears in code, docs, logs, diffs, or screenshots.
- No raw acceptance JSON or `/tmp` response content is stored.
- No fixed scenario workflow is introduced.
