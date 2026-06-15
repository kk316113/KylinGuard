# Architecture

未来目标架构：

```text
User Task
-> Go/Eino Agent Runtime
-> Intent Guard
-> Tool Registry
-> Kylin Ops Tools
-> Tool Trace
-> Python Audit Core
-> Evidence Chain
-> Final Report
```

## 当前 Stage 1 状态

- `Go/Eino Agent Runtime`：当前为 Go runtime + static planner，占位等待 Eino adapter。
- `Intent Guard`：当前为关键词规则占位。
- `Tool Registry`：当前已注册基础工具接口。
- `Kylin Ops Tools`：当前仅提供保守实现和 stub。
- `Tool Trace`：当前已定义统一 trace 字段。
- `Python Audit Core`：当前为 FastAPI 服务，通过 TraceShield adapter 调用清洗后的 TraceShield 方法核心。
- `Evidence Chain`：当前由 adapter 将 TraceShield evidence steps 转换为统一 HTTP 输出。
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

## Eino 接入边界

当前不硬编码 Eino import 路径。后续确认官方模块路径和 API 后，可在 `agent-go/internal/agent` 下新增 adapter，并保留 `Planner` interface 作为稳定边界。
