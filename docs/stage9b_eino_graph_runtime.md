# Stage 9B: CloudWeGo Eino Graph Runtime

## 目标

Stage 9B 的目标是在不破坏现有稳定链路的前提下，引入 CloudWeGo Eino graph runtime，用确定性的 tool-calling 编排验证后续真实 Agent 编排的接口边界。

当前 `/api/agent/run` 保持稳定主链路不变：

```text
intent_guard
-> Rule-based Ops Planner
-> Tool Registry
-> semantic trace
-> audit-core-py / TraceShield
```

`/api/agent/run-eino` 使用实验链路：

```text
intent_guard
-> CloudWeGo Eino compose.Graph
-> deterministic ChatModel Stub
-> MCP-like Tool Adapter
-> Tool Policy
-> Tool Registry
-> semantic trace
-> audit-core-py / TraceShield
```

## 当前实现

- 引入 `github.com/cloudwego/eino v0.9.8`。
- 使用 `compose.Graph` 构建两个节点：
  - `deterministic_chat_model_stub`
  - `mcp_like_tool_node`
- 使用 `schema.ToolCall` 表达 deterministic tool calls。
- 不接真实 ChatModel。
- 不接 ReAct Agent。
- 不接远程模型 API。
- 不引入模型厂商 SDK。
- 不读取 API key。

## Deterministic ChatModel Stub

当前固定映射：

- `检查当前系统 SSH 登录异常`
  -> `os_info`
  -> `service_status(service_name=sshd)`
  -> `port_checker(host=127.0.0.1, port=22)`
  -> `log_reader(paths=/var/log/secure,/var/log/auth.log, lines=100, purpose=ssh_login_anomaly_check)`
  -> `ssh_login_analyzer(paths=/var/log/secure,/var/log/auth.log, lines=200)`

- `检查 sshd 服务状态`
  -> `service_status(service_name=sshd)`

- `检查 22 端口是否开放`
  -> `port_checker(host=127.0.0.1, port=22)`

未知安全任务会 fallback 到现有 Rule-based Planner 生成 tool calls。Stub 不会选择 `safe_shell`。

## 安全边界

危险任务必须在 graph 之前被 `intent_guard` deny：

- 不进入 deterministic ChatModel Stub
- 不进入 MCP-like Tool Adapter
- 不执行任何工具
- 不调用 audit-core-py
- `audit_result.method=intent_guard`

Eino graph 不是安全边界。工具执行仍必须经过 Tool Policy，并且执行结果仍必须生成 semantic trace，再进入 TraceShield 审计。

## 响应标记

run-eino 的 summary：

```text
Eino graph runtime executed deterministic tool-calling orchestration.
```

`security_report.audit_metadata`：

```text
route=eino-runtime
runtime=eino
eino_graph_enabled=true
llm_enabled=false
chat_model=deterministic-stub
orchestration=eino-graph-tool-calling
tool_protocol=mcp-like
tool_protocol_version=stage8-v1
eino_runtime_version=stage9b-v1
registered_tool_count=<number>
tools_used=<tool names>
```

## 配置

默认值：

```bash
EINO_RUNTIME_ENABLED=true
EINO_GRAPH_ENABLED=true
EINO_LLM_ENABLED=false
EINO_ENABLED=false
```

`EINO_ENABLED=false` 只表示不启用真实 LLM，不会导致 run-eino fallback 到 stable runtime。

## 测试

Go 测试覆盖：

- deterministic ChatModel Stub 映射。
- Eino graph 两节点调用。
- run-eino safe task 调用 TraceShield。
- run-eino dangerous task 在 chat/tool/audit 前短路。
- Tool Policy 拒绝 unknown tool、dangerous safe_shell、非白名单日志、危险 service name、非法端口。
- handler 级 run-eino metadata。

运行：

```bash
cd agent-go
gofmt -w .
go test ./...
```

## 后续

- 在 Kylin V11 x86_64 VM 继续跑 Linux E2E。
- 在 LoongArch 环境验证 Eino core 依赖构建。
- 后续替换 deterministic ChatModel Stub 为真实 Eino ChatModel / ReAct Agent。
- 替换时不得绕过 intent_guard、Tool Policy、semantic trace、audit-core-py / TraceShield。
