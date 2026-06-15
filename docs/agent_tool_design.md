# Agent Tool Design

## Tool Registry

`agent-go/internal/tools/registry.go` 提供统一工具注册和调用入口。所有工具调用都会生成 `ToolTrace`。

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
- `service_status`：在 Linux 上探测 systemctl 状态，非 Linux 返回 not_supported。
- `log_reader`：Stage 0 stub，不读取日志文件。
- `port_checker`：检查本机或指定地址端口是否可连通。
- `safe_shell`：只执行极少数白名单命令。

## 语义映射

`agent-go/internal/tools/semantic.go` 为每个工具生成安全语义：

- `os_info`：读取公开系统信息，`boundary_level=public`。
- `port_checker`：检查本地网络端口，`boundary_level=low`。
- `service_status`：读取 systemd 服务状态，`boundary_level=low`。
- `log_reader`：读取系统日志；`/var/log/secure`、`/var/log/auth.log`、`/var/log/audit/audit.log`、`/var/log/messages`、`/var/log/syslog` 标记为 `sensitive_system_resource`。
- `safe_shell`：白名单命令标记为 `allowed_by_policy=true`；危险命令标记为 `boundary_level=dangerous` 且 `allowed_by_policy=false`。

## safe_shell 白名单

- `uname -a`
- `hostname`
- `whoami`
- `date`
- `df -h`
- `free -h`
- `systemctl --version`

`safe_shell` 不使用 shell 管道执行命令，不允许 `rm`、`shutdown`、`reboot`、`mkfs`、`dd`、`chmod 777`、`curl | sh` 等危险模式。
