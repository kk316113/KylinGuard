# Stage 8 MCP-like Tool Protocol

Stage 8 将 KylinGuard 的运维工具层从 Go 内部函数扩展为 MCP-like Tool Protocol / Plugin Registry。目标不是开放后门式工具调用，而是让工具具备可发现、可描述、可策略控制、可审计和可被后续 Eino runtime 接入的能力。

## 为什么引入

后续真实 Agent 编排需要知道有哪些工具、每个工具的输入输出结构、风险级别、权限范围和资源边界。只保留 Go 函数调用无法表达这些信息，也不利于后续 MCP 风格插件化。

Stage 8 引入 `ToolMetadata`，并新增三个 HTTP API：

- `GET /api/tools`
- `GET /api/tools/{name}`
- `POST /api/tools/call`

这三类接口对应 MCP 思想中的 `tools/list`、工具详情和 `tools/call`，但实现保持在 KylinGuard 的安全边界内。

## ToolMetadata

当前结构位于 `agent-go/internal/tools/metadata.go`：

- `name`
- `description`
- `category`
- `version`
- `input_schema`
- `output_schema`
- `risk_level`
- `permission_scope`
- `operation_type`
- `resource_type`
- `boundary_level`
- `requires_privilege`
- `allowed_by_policy`
- `dangerous`
- `enabled`
- `direct_call_allowed`

协议名为 `mcp-like`，协议版本为 `stage8-v1`。

## 已注册工具

- `os_info`：读取公开系统信息，低风险。
- `service_status`：只读检查 systemd 服务状态，低风险。
- `port_checker`：检查 TCP 端口可达性，低风险。
- `log_reader`：读取白名单系统日志，中风险，敏感系统资源。
- `ssh_login_analyzer`：分析 SSH 认证日志，中风险，敏感系统资源。
- `safe_shell`：只允许极少数白名单诊断命令，但默认不允许 direct call。

`safe_shell` 会出现在工具列表中用于说明能力边界，但 `enabled=false` 且 `direct_call_allowed=false`，避免 `/api/tools/call` 变成 shell 后门。

## /api/tools

只返回 metadata，不执行工具，不调用 audit-core-py。

示例：

```json
{
  "protocol": "mcp-like",
  "version": "stage8-v1",
  "count": 6,
  "tools": []
}
```

## /api/tools/{name}

返回单个工具 metadata，包括 schema、权限范围、资源类型和边界级别。未知工具返回：

```json
{
  "error": "tool not found",
  "tool_name": "unknown"
}
```

## /api/tools/call

请求：

```json
{
  "tool_name": "port_checker",
  "input": {
    "host": "127.0.0.1",
    "port": 22
  },
  "reason": "Stage 8 direct MCP-like tool call"
}
```

安全流程：

1. 解析请求。
2. 查询 Tool Metadata。
3. 执行 Tool Policy。
4. deny 时不执行工具，返回 `status=denied` 和 `audit_result.method=tool_policy`。
5. allow 时复用现有 Tool Registry 执行工具。
6. Registry 生成 semantic `tool_trace`。
7. 将单工具 trace 包装为 `Manual MCP-like tool call: <tool>`。
8. 调用 audit-core-py / TraceShield。
9. 返回 output、trace 和 audit_result。

工具执行失败时仍返回 trace，并尽量进入审计链路，不 panic。

## Tool Policy

Tool Policy 位于 `agent-go/internal/security/tool_policy.go`。当前规则：

- unknown tool：deny。
- `enabled=false`：deny。
- `direct_call_allowed=false`：deny。
- `dangerous=true`：deny。
- `allowed_by_policy=false`：deny。
- `safe_shell`：默认 deny direct call。
- `port_checker`：端口必须在 `1-65535`。
- `log_reader`：只能读取白名单日志路径。
- `ssh_login_analyzer`：只能读取白名单认证日志路径或使用 `journalctl:sshd` 语义。
- `service_status`：服务名只允许字母、数字、下划线、短横线和点号。

## 为什么不是后门

`/api/tools/call` 不允许任意 shell，不允许任意文件读取，也不允许跳过审计。它只暴露已注册工具，且每次 direct call 都必须经过 Tool Policy、semantic trace 和 audit-core-py / TraceShield 或 fallback 审计链路。

前端不是安全边界。即使未来前端展示工具列表，实际安全约束仍在 Go Agent 服务端。

## 与 TraceShield 的关系

Stage 8 不修改 TraceShield-Core，也不改变 TraceShield 算法语义。Go Agent 仍通过 `auditclient` 调用 audit-core-py 的 `/audit/trace`。单工具调用只是构造一个最小 synthetic task 和一条 semantic trace。

如果 audit-core-py 不可用，`auditclient` 会返回 `method=fallback-mock` 并在 message 中说明原因，不会伪装成 TraceShield 成功。

## 与 Eino 的关系

当前仍不接入真实 Eino runtime，也不在 `go.mod` 中加入 Eino 依赖。Stage 8 的 metadata 和 registry 是为了后续 Eino runtime 可以发现工具、理解 schema、读取风险边界，并复用同一套 Tool Policy 和审计链路。

## 当前限制

- 没有 MCP 标准传输层，只是 MCP-like HTTP API。
- 没有工具 marketplace。
- 没有动态加载外部插件。
- 没有开放 `safe_shell` direct call。
- 没有新增前端工具管理页面。
- 没有做 LoongArch 验证。

## 下一步

- 在银河麒麟 V11 VM 上验证 `/api/tools` 与 `/api/tools/call`。
- 扩展更多只读安全诊断工具 metadata。
- 将 ToolMetadata 接入未来真实 Eino runtime。
- 为报告导出和 risk graph 展示补充更细粒度的工具审计信息。
