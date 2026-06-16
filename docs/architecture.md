# Architecture

未来目标架构：

```text
User Task
-> Go/Eino Agent Runtime
-> Intent Guard
-> Rule-based Ops Planner
-> Tool Registry
-> Kylin Ops Tools
-> Semantic Tool Trace
-> Python Audit Core
-> Evidence Chain
-> Final Report
```

## 当前 Stage 4 状态

- `Go/Eino Agent Runtime`：稳定主链路为 Go runtime + Rule-based Ops Planner；默认禁用的 Eino adapter 仍 fallback 到稳定 runtime。
- `Intent Guard`：当前为关键词规则占位。
- `Rule-based Ops Planner`：根据任务选择 `ssh_anomaly_check`、`service_check`、`port_check`、`system_overview` 等工具计划。
- `Tool Registry`：当前已注册基础工具接口，并按 Plan 步骤执行。
- `Kylin Ops Tools`：当前提供保守实现，不允许任意 shell 执行或任意文件读取。
- `Tool Trace`：当前已定义统一 trace 字段，并携带工具语义、资源语义、权限范围和边界级别。
- `Python Audit Core`：当前为 FastAPI 服务，通过 TraceShield adapter 调用清洗后的 TraceShield 方法核心。
- `Evidence Chain`：当前由 adapter 将 TraceShield evidence steps 转换为统一 HTTP 输出，并将语义 trace 转换为 `risk_graph.nodes`。
- `Final Report`：当前未实现。

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
-> Tool Registry
Kylin Ops Tool
-> tools.SemanticForTool(...)
-> logtrace.ToolTrace semantic fields
-> auditclient HTTP payload
-> audit-core-py ToolTraceStep
-> TraceShieldAdapter normalized ToolEvent args
-> risk_graph semantic nodes
```

## Eino 接入边界

当前不硬编码 Eino import 路径，也不引入 Eino 外部依赖。`/api/agent/run` 是稳定主链路，`/api/agent/run-eino` 是实验链路。Eino adapter 未启用或真实 runtime 未实现时，handler fallback 到 `StableRuntimeAdapter`，因此仍会执行 `intent_guard`、Tool Registry、语义 trace 和 audit-core-py。

Stage 4 之后，`/api/agent/run-eino` fallback 也会复用 Rule-based Ops Planner，不会退回旧静态工具链。
