# Architecture

> Current status: this document was originally written during the Stage 9B
> transition. The production architecture has since converged on the
> LLM-driven Agent Loop as the main runtime. Historical notes below are kept for
> context, but the authoritative current state is in `AGENTS.md` and
> `docs/agent_memory/CURRENT_STATE.md`.

KylinGuard 当前采用 Agent Loop 主链路 + 兼容接口的架构。

生产主链路：

```text
User Task
-> Go /api/agent/run
-> Intent Guard
-> LLM-driven Agent Loop
-> structured next_action schema validation
-> Tool Policy
-> Exec Proxy / Tool Registry
-> read-only Kylin Ops Tools
-> observation
-> Semantic Tool Trace
-> Python Audit Core
-> TraceShield Adapter/Core
-> Security Report / Risk Graph
-> final_answer
-> Next.js Agent Console
```

兼容链路：

```text
User Task
-> Go /api/agent/run-eino
-> Intent Guard
-> same Agent Loop runtime as /api/agent/run
-> structured next_action schema validation
-> Tool Policy
-> Exec Proxy / Tool Registry
-> observation / trace / audit / final_answer
```

## 当前状态

- `/api/agent/run` 是当前主接口，使用 Agent Loop adapter。
- `/api/agent/run-eino` 保留为兼容路径，调用同一个 Agent Loop handler。
- 真实 DeepSeek / OpenAI-compatible 远程 LLM 已接入；无 key 时使用 deterministic baseline 作为回归和安全降级。
- Agent Loop 只接受结构化 `next_action`，系统侧执行 schema 校验、Intent Guard、Tool Policy 和 Exec Proxy。
- MCP 已从早期 “MCP-like” 工具协议演进为标准 Streamable HTTP `/mcp` endpoint，同时保留 `/api/tools*` 管理接口。
- `safe_shell` 不向 Agent/MCP 暴露；没有任意 shell 或任意文件读取。
- `configuration_drift_detector` 已通过 RPM 数据库和严格 `rpm --verify <package>` 参数形态实现只读配置漂移检测。
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

如果 TraceShield import 或运行失败，adapter 使用明确标记的本地安全降级结果，不伪造 TraceShield 结论。

## MCP-like Tool Protocol

工具协议提供：

- `GET /api/tools`：列出工具 metadata，不执行工具。
- `GET /api/tools/{name}`：返回单个工具 schema 和权限边界。
- `POST /api/tools/call`：受 Tool Policy 控制的单工具调用；允许后仍会生成 semantic trace 并进入 audit-core-py / TraceShield 审计。
- `POST /mcp`：标准 MCP Streamable HTTP endpoint，支持 initialize、tools/list、tools/call。

它不是绕过 Agent 的后门。`safe_shell` 不对 Agent/MCP 暴露；日志、进程、端口、RPM 漂移等能力均受 Tool Policy 和 Exec Proxy 约束。

## Eino Metadata

Agent Loop 响应会在 `security_report.audit_metadata` 中记录：

- `route=eino-runtime`
- `runtime=eino`
- `eino_graph_enabled=true`
- `llm_enabled`
- `remote_llm_used`
- `chat_model`
- `fallback_reason`
- `orchestration`
- `tool_protocol`
- `tool_protocol_version=stage8-v1`
- `eino_runtime_version`
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
