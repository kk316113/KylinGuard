# Architecture

KylinGuard 当前采用稳定主链路 + 实验 Eino 链路并行演进的架构。

稳定主链路：

```text
User Task
-> Go Agent Runtime
-> Intent Guard
-> Rule-based Ops Planner
-> MCP-like Tool Registry
-> Kylin Ops Tools
-> SSH Diagnosis Tools
-> Semantic Tool Trace
-> Python Audit Core
-> TraceShield Adapter/Core
-> Evidence Chain
-> Report Builder
-> Frontend Security Console
```

Stage 9B 实验链路：

```text
User Task
-> Go /api/agent/run-eino
-> Intent Guard
-> CloudWeGo Eino compose.Graph
-> Deterministic ChatModel Stub
-> MCP-like Tool Adapter
-> Tool Policy
-> MCP-like Tool Registry
-> Kylin Ops Tools
-> Semantic Tool Trace
-> Python Audit Core
-> TraceShield Adapter/Core
-> Evidence Chain
-> Report Builder
```

## 当前状态

- `/api/agent/run` 仍是稳定 Go runtime，不被 Stage 9B 改写。
- `/api/agent/run-eino` 已进入 CloudWeGo Eino graph runtime，不再 fallback 到 stable runtime。
- Eino graph 第一节点是 deterministic ChatModel Stub，负责把任务映射为确定性的 tool calls。
- Eino graph 第二节点是 MCP-like Tool Adapter，负责通过 Tool Policy 和 Tool Registry 执行工具。
- 当前没有真实 LLM、远程模型 API、API key、模型厂商 SDK 或 ReAct Agent。
- `audit-core-py` 仍是唯一的 TraceShield 入口，Go Agent 不直接 import TraceShield-Core。
- `security_report` 由 Go 侧 report builder 生成，只解释结果，不改变最终 decision。

## TraceShield 接入边界

当前采用 `audit-core-py` 读取 `TRACESHIELD_CORE_PATH` 的方式接入 TraceShield：

```text
Go Agent
-> POST ${AUDIT_CORE_URL}/audit/trace
-> audit-core-py TraceShieldAdapter
-> traceshield_experiment_core.TraceShieldEvaluator
```

如果 TraceShield import 或运行失败，adapter 会返回 fallback mock，并在 `message` 中说明原因。

## MCP-like Tool Protocol

Stage 8 工具协议提供：

- `GET /api/tools`：列出工具 metadata，不执行工具。
- `GET /api/tools/{name}`：返回单个工具 schema 和权限边界。
- `POST /api/tools/call`：受 Tool Policy 控制的单工具调用；允许后仍会生成 semantic trace 并进入 audit-core-py / TraceShield 审计。

它不是绕过 Agent 的后门。`safe_shell` 默认禁止 direct call；`log_reader` 和 `ssh_login_analyzer` 只能访问白名单日志路径。

## Eino Metadata

Stage 9B run-eino 响应会在 `security_report.audit_metadata` 中记录：

- `route=eino-runtime`
- `runtime=eino`
- `eino_graph_enabled=true`
- `llm_enabled=false`
- `chat_model=deterministic-stub`
- `orchestration=eino-graph-tool-calling`
- `tool_protocol=mcp-like`
- `tool_protocol_version=stage8-v1`
- `eino_runtime_version=stage9b-v1`
- `registered_tool_count`
- `tools_used`

## 安全边界

Eino 不是安全边界。run-eino 仍必须经过：

```text
intent_guard
-> Tool Policy
-> semantic trace
-> audit-core-py / TraceShield
```

危险任务会在 Eino graph 和 deterministic ChatModel Stub 之前被 deny，不生成 plan，不执行工具，不调用 audit-core-py。

## Frontend 边界

前端只调用 Go Agent：

- `GET /health`
- `POST /api/agent/run`
- `POST /api/agent/run-eino`

前端不直接调用 audit-core-py，不执行系统命令，不决定 `allow/review/deny`，也不提供删除日志、封禁 IP、关闭防火墙或重启服务等自动处置按钮。
