# Architecture

未来目标架构：

```text
User Task
-> Go/Eino Agent Runtime
-> Intent Guard
-> Rule-based Ops Planner
-> MCP-like Tool Registry
-> Kylin Ops Tools
-> SSH Diagnosis Tools
-> Semantic Tool Trace
-> Python Audit Core
-> Evidence Chain
-> Report Builder
-> Frontend Security Console
```

## 当前 Stage 8 状态

- `Go/Eino Agent Runtime`：稳定主链路为 Go runtime + Rule-based Ops Planner + SSH diagnosis tools + Report Builder；默认禁用的 Eino adapter 仍 fallback 到稳定 runtime。
- `Intent Guard`：当前为关键词规则占位。
- `Rule-based Ops Planner`：根据任务选择 `ssh_anomaly_check`、`service_check`、`port_check`、`system_overview` 等工具计划。
- `MCP-like Tool Registry`：当前已注册基础工具 metadata 和 executor，并提供 `/api/tools`、`/api/tools/{name}`、`/api/tools/call`。
- `Tool Policy`：控制 direct tool call，禁止 unknown tool、`safe_shell` direct call、越界端口、非白名单日志和恶意 service name。
- `Kylin Ops Tools`：当前提供保守实现，不允许任意 shell 执行或任意文件读取。
- `SSH Diagnosis Tools`：`auth_log_collector` 和 `ssh_login_analyzer` 只读采集并分析 SSH 认证日志，输出 `diagnosis`。
- `Tool Trace`：当前已定义统一 trace 字段，并携带工具语义、资源语义、权限范围和边界级别。
- `Python Audit Core`：当前为 FastAPI 服务，通过 TraceShield adapter 调用清洗后的 TraceShield 方法核心。
- `Evidence Chain`：当前由 adapter 将 TraceShield evidence steps 转换为统一 HTTP 输出，并将语义 trace 转换为 `risk_graph.nodes`。
- `Report Builder`：当前在 Go Agent 侧生成 `security_report`，用于解释 plan、tool_trace、diagnosis 和 audit_result。
- `Frontend Security Console`：当前为 Vue 3 + Vite 单页控制台，展示任务输入、执行计划、审计摘要、证据链、敏感资源、风险解释、建议和 Raw JSON。

## TraceShield 接入边界

当前采用策略 A：`audit-core-py` 读取 `TRACESHIELD_CORE_PATH`，将 `D:\code\2026\TraceShield-Core` 或部署环境中的等价路径加入 `sys.path`，然后调用 `traceshield_experiment_core.TraceShieldEvaluator`。

Go/Eino Agent 永远只调用 `audit-core-py` 的 HTTP API：

```text
Go Agent
-> POST ${AUDIT_CORE_URL}/audit/trace
-> audit-core-py TraceShieldAdapter
-> traceshield_experiment_core.TraceShieldEvaluator
```

如果 TraceShield import 或运行失败，adapter 会返回 fallback mock，并在 `message` 中写明原因。

## 工具语义流

```text
Rule-based Plan
-> PlanStep
-> MCP-like Tool Registry
-> Tool Policy for direct calls
Kylin Ops Tool
-> SSH Diagnosis Tool
-> tools.SemanticForTool(...)
-> logtrace.ToolTrace semantic fields
-> auditclient HTTP payload
-> audit-core-py ToolTraceStep
-> TraceShieldAdapter normalized ToolEvent args
-> risk_graph semantic nodes
-> report.BuildSecurityReport(...)
-> security_report.audit_metadata tool_protocol metadata
-> security_report.evidence_chain / risk_explanation / recommendations
-> frontend security console
```

## MCP-like Tool Protocol 边界

Stage 8 的工具协议提供：

- `GET /api/tools`：只列出工具 metadata，不执行工具。
- `GET /api/tools/{name}`：只返回单个工具 schema 和权限边界。
- `POST /api/tools/call`：受 Tool Policy 控制的单工具调用，允许后仍会生成 semantic trace 并进入 audit-core-py / TraceShield 审计。

它不是绕过 Agent 的后门。`safe_shell` 默认禁止 direct call，`log_reader` 和 `ssh_login_analyzer` 仍只能访问白名单日志路径。前端即使展示这些接口，也不构成安全边界。

## Frontend 边界

前端只调用 Go Agent：

- `GET /health`
- `POST /api/agent/run`
- `POST /api/agent/run-eino`

前端不直接调用 audit-core-py，不执行系统命令，不决定 `allow/review/deny`，也不提供删除日志、封禁 IP、关闭防火墙或重启服务等自动处置按钮。

## Eino 接入边界

当前不硬编码 Eino import 路径，也不引入 Eino 外部依赖。`/api/agent/run` 是稳定主链路，`/api/agent/run-eino` 是实验链路。Eino adapter 未启用或真实 runtime 未实现时，handler fallback 到 `StableRuntimeAdapter`，因此仍会执行 `intent_guard`、Tool Registry、语义 trace 和 audit-core-py。

Stage 6 之后，`/api/agent/run-eino` fallback 也会复用 Rule-based Ops Planner、SSH diagnosis tools 和 Report Builder，不会退回旧静态工具链。报告中的 `audit_metadata.route=eino-fallback` 用于说明 fallback 未绕过安全链路。

## Report 边界

`security_report` 只解释，不裁决。最终 `decision` 仍来自 intent_guard、TraceShield 和现有 decision flow。Report Builder 不接 LLM，不执行建议，也不修改任何系统状态。
