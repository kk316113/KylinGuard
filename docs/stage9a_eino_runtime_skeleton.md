# Stage 9A Eino Runtime Skeleton

Stage 9A 将 `/api/agent/run-eino` 从 stable runtime fallback 升级为 deterministic Eino Runtime Skeleton。当前不接真实 LLM、不接外部 API、不要求 API key，也不引入 CloudWeGo Eino 依赖。

## 目标

本阶段目标是建立 Eino-compatible 的编排骨架，让后续真实 Eino ChatModel / ReAct Agent 能接入同一套工具协议、安全策略和审计链路。

当前链路：

```text
User Task
-> intent_guard
-> Eino Runtime Skeleton
-> Rule-based Planner
-> MCP-like Tool Adapter
-> Tool Registry
-> Tool Policy
-> Kylin Ops Tools
-> semantic tool_trace
-> audit-core-py
-> TraceShield
-> diagnosis
-> security_report
```

## 为什么不直接接 LLM

比赛工程当前优先保证安全链路可验证。直接接 LLM 会引入 API key、模型行为不确定性和额外故障面，不利于验证 intent_guard、Tool Policy、TraceShield 和 Kylin 工具链是否稳定。

Stage 9A 因此只实现 deterministic planner-backed orchestration：

- `llm_enabled=false`
- 不调用远程模型
- 不运行本地大模型
- 不改变 TraceShield 语义
- 不修改 TraceShield-Core

## Runtime Skeleton

新增包：

```text
agent-go/internal/eino/
```

核心文件：

- `types.go`：RuntimeConfig 和 RuntimeMetadata。
- `runtime.go`：Eino Runtime Skeleton 主流程。
- `tool_adapter.go`：MCP-like Tool Adapter。
- `adapter.go`：声明 Runtime 实现 `agent.AgentAdapter`。

默认 metadata：

- `route=eino-runtime`
- `runtime=eino`
- `llm_enabled=false`
- `orchestration=deterministic-planner-backed`
- `tool_protocol=mcp-like`
- `eino_runtime_version=stage9a-v1`

## MCP-like Tool Adapter

Tool Adapter 将 planner step 转换成受控工具调用：

1. 查询 `ToolMetadata`。
2. 执行 `ToolPolicy`。
3. allow 后调用现有 `Tool Registry`。
4. 复用现有工具实现。
5. 生成 semantic `tool_trace`。
6. 工具失败时返回 error trace，不 panic。

Tool Adapter 不直接执行 shell，不直接读取任意文件，也不绕过 Stage 8 的工具协议。

## 安全边界

Eino 不是安全边界。安全仍由以下部分共同保证：

- `intent_guard`：危险任务在工具执行前 deny。
- `Tool Policy`：控制单工具执行边界。
- `Tool Registry`：只允许已注册工具。
- `safe_shell`：默认禁止 direct call。
- `log_reader`：只允许白名单日志路径。
- `audit-core-py / TraceShield`：审计完整 semantic tool trace。

## /api/agent/run 与 /api/agent/run-eino

`/api/agent/run` 仍是 stable runtime：

```text
intent_guard -> Rule-based Planner -> Tool Registry -> TraceShield -> security_report
```

`/api/agent/run-eino` 当前是 Stage 9A runtime skeleton：

```text
intent_guard -> Eino Runtime Skeleton -> Rule-based Planner -> MCP-like Tool Adapter -> Tool Registry -> Tool Policy -> TraceShield -> security_report
```

`/api/agent/run-eino` 不再返回 `stable runtime fallback used`。

## 配置

新增或支持：

```bash
EINO_RUNTIME_ENABLED=true
EINO_LLM_ENABLED=false
```

默认：

- `EINO_RUNTIME_ENABLED=true`
- `EINO_LLM_ENABLED=false`

旧的 `EINO_ENABLED=false` 只作为兼容含义保留，可理解为不启用真实 LLM，不会导致 `/api/agent/run-eino` fallback 到 stable runtime。

## 当前限制

- 未接真实 CloudWeGo Eino dependency。
- 未接真实 ChatModel。
- 未接 ReAct Agent。
- 未接远程 LLM API。
- 未新增前端页面。
- 未做 LoongArch 验证。

## Stage 9B 计划

后续 Stage 9B 可以在保持安全边界不变的前提下接入真实 Eino：

- 明确 CloudWeGo Eino module path 和版本。
- 用 build tag 或 adapter 替换方式引入真实 runtime。
- 接入远程 ChatModel。
- 将 tool metadata 映射为真实 Eino tools。
- 保持 intent_guard、Tool Policy 和 TraceShield 不可绕过。
