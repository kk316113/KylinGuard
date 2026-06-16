# agent-go

`agent-go` 是 KylinGuard 的 Go Agent 后端，module 名称为 `kylin-guard-agent/agent-go`。它负责接收用户任务、执行安全意图护栏、调用运维工具、生成 semantic trace、调用 `audit-core-py` / TraceShield，并返回统一的审计响应。

## 启动

```bash
go run ./cmd/server
```

默认监听 `:8080`。可以通过 `KYLIN_GUARD_AGENT_PORT` 或 `KYLIN_GUARD_AGENT_ADDR` 覆盖。

常用环境变量：

```bash
export AUDIT_CORE_URL=http://127.0.0.1:8001
export KYLIN_GUARD_AGENT_PORT=8080
export EINO_RUNTIME_ENABLED=true
export EINO_GRAPH_ENABLED=true
export EINO_LLM_ENABLED=false
export EINO_ENABLED=false
```

`EINO_ENABLED=false` 只保留兼容含义，表示不启用真实 LLM；它不会让 `/api/agent/run-eino` fallback 到 stable runtime。

## 接口

- `GET /health`
- `GET /api/os/info`
- `POST /api/agent/run`
- `POST /api/agent/run-eino`
- `GET /api/tools`
- `GET /api/tools/{name}`
- `POST /api/tools/call`

`/api/agent/run` 是稳定主链路：

```text
intent_guard
-> Rule-based Ops Planner
-> Tool Registry
-> Kylin Ops Tools
-> semantic tool trace
-> audit-core-py / TraceShield
-> security_report
```

`/api/agent/run-eino` 是 Stage 9B 实验链路：

```text
intent_guard
-> CloudWeGo Eino compose.Graph
-> deterministic ChatModel Stub
-> MCP-like Tool Adapter / Tool Policy
-> Tool Registry
-> semantic tool trace
-> audit-core-py / TraceShield
-> security_report
```

run-eino 的 `security_report.audit_metadata` 会包含：

- `route=eino-runtime`
- `runtime=eino`
- `eino_graph_enabled=true`
- `llm_enabled=false`
- `chat_model=deterministic-stub`
- `orchestration=eino-graph-tool-calling`
- `tool_protocol=mcp-like`
- `tool_protocol_version=stage8-v1`
- `eino_runtime_version=stage9b-v1`

## 当前 Eino 状态

当前已引入 CloudWeGo Eino core 依赖：

```text
github.com/cloudwego/eino v0.9.8
```

只使用 Eino `compose.Graph` / `schema` 相关能力，不接真实 ChatModel、ReAct Agent、模型厂商 SDK、API key 或远程模型调用。

Deterministic ChatModel Stub 当前固定映射：

- `检查当前系统 SSH 登录异常` -> `os_info`、`service_status(sshd)`、`port_checker(22)`、`log_reader`、`ssh_login_analyzer`
- `检查 sshd 服务状态` -> `service_status(sshd)`
- `检查 22 端口是否开放` -> `port_checker(22)`

危险任务仍在 graph 和 chat stub 之前被 `intent_guard` deny，不会执行工具，也不会调用 audit-core-py。

## 开发测试

```bash
gofmt -w .
go test ./...
```

## 安全边界

- `intent_guard` 永远在 planner / Eino graph / tool execution 之前运行。
- `safe_shell` 不允许 direct call，Eino stub 也不会选择 `safe_shell`。
- `log_reader` 和 `ssh_login_analyzer` 只读取白名单日志路径。
- `MCPToolAdapter` 必须经过 Tool Policy。
- `security_report` 只解释现有结果，不负责裁决。
- Go Agent 不直接 import TraceShield-Core，只通过 `audit-core-py` HTTP API 调用。
