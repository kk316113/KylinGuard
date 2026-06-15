# agent-go

Go Agent 最小服务骨架，module 名为 `kylin-guard-agent/agent-go`。

## 启动

```bash
go run ./cmd/server
```

默认监听 `:8080`。可通过 `KYLIN_GUARD_AGENT_PORT` 或 `KYLIN_GUARD_AGENT_ADDR` 覆盖。审计服务默认调用 `AUDIT_CORE_URL=http://127.0.0.1:8001`。

## 接口

`GET /health`

```json
{
  "status": "ok",
  "service": "kylin-guard-agent",
  "version": "0.1.0"
}
```

`GET /api/os/info`

返回当前系统基础信息：`os`、`arch`、`hostname`、可选 `kernel`、`timestamp`。

`POST /api/agent/run`

```json
{
  "task": "检查当前系统状态"
}
```

返回 agent 结果，包含 `tool_trace` 和来自 `audit-core-py` 的 `audit_result`。如果 `audit-core-py` 不可用，会回退到本地 mock 审计结果。

## Eino 接入说明

当前阶段没有硬编码 Eino import 路径。`internal/agent/planner.go` 只提供 `Planner` interface 与 `StaticPlanner` 占位实现。后续确认 Eino 的实际模块路径和 API 后，再新增 adapter 接入。

## 安全边界

- `safe_shell` 只允许极少数白名单命令。
- `intent_guard` 只做最小危险关键词拦截。
- `auditclient` 当前是 mock 返回。
- 不包含论文审计方法和重依赖。
