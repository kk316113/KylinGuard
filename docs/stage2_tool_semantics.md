# Stage 2 Tool Semantics

Stage 2 将 Go Agent 的运维工具 trace 从简单调用记录扩展为可审计语义事件。这样 TraceShield adapter 可以看到工具动作、资源类型、权限范围、边界级别和策略原因，并把这些信息输出到 `risk_graph.nodes`。

## 新增字段

- `operation_type`
- `resource_type`
- `resource_path`
- `permission_scope`
- `boundary_level`
- `tool_semantic`
- `requires_privilege`
- `allowed_by_policy`
- `policy_reason`

## 工具语义

- `os_info`：`read` / `os_info` / `system:os` / `public_system_info` / `public`
- `port_checker`：`inspect` / `network_port` / `tcp:{host}:{port}` / `network_port_inspect` / `low`
- `service_status`：`inspect` / `system_service` / `systemd:{service}` / `service_status_read` / `low`
- `log_reader`：`read` / `system_log` / `{path}` / `system_log_read` / `sensitive_system_resource` for sensitive logs
- `safe_shell`：白名单命令是 `safe_command_execute` + `low`；危险命令是 `privileged_command_execute` + `dangerous`

## Guard 与 TraceShield 分工

`intent_guard` 在工具执行前判断用户目标是否明显危险，命中后直接 deny，不执行工具、不调用 audit-core-py。

TraceShield 审计已经执行或计划执行的工具调用链，结合语义字段生成风险决策、证据链和 risk graph。

## Risk Graph

当前 adapter 为每个 tool step 生成一个 node，包含 `step_id`、`tool_name`、`operation_type`、`resource_type`、`resource_path`、`boundary_level`、`risk_hint`、`status` 和 `allowed_by_policy`。edges 当前按 step 顺序连接，类型为 `sequence`。

## TODO

- 继续扩展真实 Kylin 运维工具集合。
- 将更多 TraceShield 原生 violation 信息映射为用户可读证据。
- 在银河麒麟 V11 和 LoongArch 环境验证脚本与依赖。
- 为 service 管理、日志读取和进程检查增加更细粒度策略。
