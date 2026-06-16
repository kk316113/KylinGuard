# Agent Tool Design

## Tool Registry

`agent-go/internal/tools/registry.go` 提供统一工具注册和调用入口。所有工具调用都会生成 `ToolTrace`。

Stage 4 之后，Tool Registry 支持按 `PlanStep` 执行，并可沿用 `plan-001` 这类 step id，便于将 Plan、tool trace 和审计证据对应起来。工具执行失败时不会让 runtime panic，而是生成 `status=error` 的 trace 并继续送入 audit-core-py 审计。

## Tool Trace 字段

- `step_id`
- `tool_name`
- `input`
- `output_summary`
- `status`
- `started_at`
- `finished_at`
- `risk_hint`
- `operation_type`
- `resource_type`
- `resource_path`
- `permission_scope`
- `boundary_level`
- `tool_semantic`
- `requires_privilege`
- `allowed_by_policy`
- `policy_reason`

## 当前工具

- `os_info`：返回当前系统基础信息。
- `service_status`：在 Linux 上使用只读 `systemctl` 子命令探测服务状态，非 Linux 返回 graceful unsupported。
- `log_reader`：按白名单读取系统日志，支持 `paths` 顺序尝试，路径或权限不可用时返回 graceful error trace。
- `port_checker`：检查本机或指定地址端口是否可连通。
- `safe_shell`：只执行极少数白名单命令。

## 语义映射

`agent-go/internal/tools/semantic.go` 为每个工具生成安全语义：

- `os_info`：读取公开系统信息，`boundary_level=public`。
- `port_checker`：检查本地网络端口，`boundary_level=low`。
- `service_status`：读取 systemd 服务状态，`resource_type=system_service`，`boundary_level=low`。
- `log_reader`：读取白名单系统日志，`resource_type=system_log`，`boundary_level=sensitive_system_resource`。
- `safe_shell`：白名单命令标记为 `allowed_by_policy=true`；危险命令标记为 `boundary_level=dangerous` 且 `allowed_by_policy=false`。

## Rule-based Ops Planner

`agent-go/internal/agent/planner.go` 当前提供规则规划器，支持：

- `ssh_anomaly_check`：`os_info -> service_status(sshd) -> port_checker(22) -> log_reader(/var/log/secure,/var/log/auth.log)`。
- `service_check`：`os_info -> service_status(service_name)`。
- `port_check`：`os_info -> port_checker(port)`。
- `system_overview`：`os_info -> port_checker(8080)`。

Planner 在 `intent_guard` 之后运行。危险任务不会进入 planner。

## log_reader 白名单

- `/var/log/secure`
- `/var/log/auth.log`
- `/var/log/messages`
- `/var/log/syslog`
- `/var/log/audit/audit.log`

`lines` 默认 `100`，最大 `500`。`/var/log/secure`、`/var/log/auth.log`、`/var/log/audit/audit.log` 标记为 `requires_privilege=true`。

## safe_shell 白名单

- `uname -a`
- `hostname`
- `whoami`
- `date`
- `df -h`
- `free -h`
- `systemctl --version`

`safe_shell` 不使用 shell 管道执行命令，不允许 `rm`、`shutdown`、`reboot`、`mkfs`、`dd`、`chmod 777`、`curl | sh` 等危险模式。
