# agent-go

Go Agent 最小服务骨架，module 名为 `kylin-guard-agent/agent-go`。

## 启动

```bash
go run ./cmd/server
```

默认监听 `:8080`。可通过 `KYLIN_GUARD_AGENT_PORT` 或 `KYLIN_GUARD_AGENT_ADDR` 覆盖。审计服务默认调用 `AUDIT_CORE_URL=http://127.0.0.1:8001`。

`EINO_ENABLED` 默认 `false`。Stage 3 只提供 Eino adapter 骨架，不引入真实 Eino 依赖。Stage 8 之后，稳定 runtime 使用 Rule-based Ops Planner、SSH 登录异常诊断工具链、MCP-like Tool Registry 和 deterministic report builder，`/api/agent/run-eino` fallback 也复用同一路径。

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

返回 agent 结果，包含 `plan`、`tool_trace`、可选 `diagnosis`、`security_report` 和来自 `audit-core-py` 的 `audit_result`。如果 `audit-core-py` 不可用，会回退到本地 mock 审计结果。

`POST /api/agent/run-eino`

实验接口。当前 Eino adapter 未启用时 fallback 到稳定 runtime，返回结构与 `/api/agent/run` 相同，并在 `summary` 中标记 `eino adapter disabled, stable runtime fallback used`。

`GET /api/tools`

返回 MCP-like 工具发现结果，只包含 metadata，不执行工具：

```json
{
  "protocol": "mcp-like",
  "version": "stage8-v1",
  "count": 6,
  "tools": []
}
```

`GET /api/tools/{name}`

返回单个工具的 `ToolMetadata`，包含 `input_schema`、`output_schema`、`permission_scope`、`operation_type`、`resource_type`、`boundary_level` 等字段。未知工具返回 HTTP 404。

`POST /api/tools/call`

受 Tool Policy 控制的单工具调用入口。允许时复用现有 `Tool Registry` 执行工具、生成 semantic trace，并以 `Manual MCP-like tool call: <tool>` 作为 synthetic task 调用 audit-core-py / TraceShield。拒绝时返回 `status=denied` 和 `audit_result.method=tool_policy`，不会执行工具。

`safe_shell` 默认 `enabled=false` 且 `direct_call_allowed=false`，不能通过 `/api/tools/call` 直连。

## Rule-based Ops Planner

当前支持：

- `ssh_anomaly_check`：`os_info -> service_status(sshd) -> port_checker(22) -> log_reader(/var/log/secure,/var/log/auth.log) -> ssh_login_analyzer`。
- `service_check`：`os_info -> service_status(service_name)`。
- `port_check`：`os_info -> port_checker(port)`。
- `system_overview`：`os_info -> port_checker(8080)`。

`intent_guard` 永远在 planner 之前运行。危险任务会直接返回 `decision=deny`，不会生成 plan，不会执行工具，也不会调用 audit-core-py。
Planner 生成计划时会查询 Tool Registry，并把工具的 `tool_category`、`risk_level`、`permission_scope` 补充到每个 plan step。

## SSH Diagnosis

`ssh_login_analyzer` 会按 `/var/log/secure`、`/var/log/auth.log`、`journalctl -u sshd` 的顺序采集认证日志，并分析：

- `Failed password`
- `Failed password for invalid user`
- `Invalid user`
- `Accepted password`
- `Accepted publickey`
- `authentication failure`

响应中的 `diagnosis` 包含 `scenario`、`risk_level`、`findings`、`recommendations` 和诊断明细。`diagnosis` 不覆盖 `audit_result`，最终 `decision` 仍来自现有审计流程。

## Security Report

`security_report` 由 `internal/report` 中的确定性 Report Builder 生成，包含：

- `evidence_chain`
- `risk_explanation`
- `sensitive_resources`
- `recommendations`
- `audit_metadata`

`security_report.overall_decision` 始终等于响应的 `decision`。报告只做解释，不负责最终裁决。
`security_report.audit_metadata` 会记录 `tool_protocol=mcp-like`、`tool_protocol_version=stage8-v1`、`registered_tool_count` 和 `tools_used`。

`tool_trace` 已包含 Stage 2 工具语义字段：

- `operation_type`
- `resource_type`
- `resource_path`
- `permission_scope`
- `boundary_level`
- `tool_semantic`
- `requires_privilege`
- `allowed_by_policy`
- `policy_reason`

## Eino 接入说明

当前阶段没有硬编码 Eino import 路径，也没有把 Eino 外部依赖加入 `go.mod`。`internal/agent/eino_adapter.go` 只提供默认禁用的 adapter 骨架。后续确认 Eino 的实际模块路径、版本和 Kylin 构建方式后，再通过 build tag 或替换 adapter 实现接入。

## 安全边界

- `safe_shell` 只允许极少数白名单命令。
- `log_reader` 只允许读取白名单 `/var/log/*` 路径，最多读取 500 行。
- `ssh_login_analyzer` 只做只读采集与分析，不封禁 IP，不修改防火墙，不删除日志。
- `security_report` 不执行任何建议，不覆盖 `audit_result`，不改变 `decision`。
- `service_status` 只执行只读 systemctl 查询命令。
- `intent_guard` 只做最小危险关键词拦截。
- `auditclient` 默认通过 HTTP 调用 `audit-core-py`，服务不可用时才 fallback mock。
- Go Agent 不直接 import TraceShield，也不包含本地大模型和重依赖。
