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

## 当前 Stage 0 状态

- `Go/Eino Agent Runtime`：当前为 Go runtime + static planner，占位等待 Eino adapter。
- `Intent Guard`：当前为关键词规则占位。
- `Tool Registry`：当前已注册基础工具接口。
- `Kylin Ops Tools`：当前仅提供保守实现和 stub。
- `Tool Trace`：当前已定义统一 trace 字段。
- `Python Audit Core`：当前为 FastAPI stub。
- `Evidence Chain`：当前为空数组 mock。
- `Final Report`：当前未实现。

## Eino 接入边界

当前不硬编码 Eino import 路径。后续确认官方模块路径和 API 后，可在 `agent-go/internal/agent` 下新增 adapter，并保留 `Planner` interface 作为稳定边界。
