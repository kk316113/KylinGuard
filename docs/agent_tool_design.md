# Agent Tool Design

## Tool Registry

`agent-go/internal/tools/registry.go` 提供统一工具注册和调用入口。所有工具调用都会生成 `ToolTrace`。

Stage 6 之后，Tool Registry 支持按 `PlanStep` 执行，并可沿用 `plan-001` 这类 step id，便于将 Plan、tool trace、diagnosis、security_report 和审计证据对应起来。工具执行失败时不会让 runtime panic，而是生成 `status=error` 的 trace 并继续送入 audit-core-py 审计。

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
- `auth_log_collector`：`ssh_login_analyzer` 内部使用的认证日志采集模块，按文件日志和 journalctl 顺序尝试。
- `ssh_login_analyzer`：分析 SSH 登录失败、无效用户、成功登录和来源 IP，输出结构化 diagnosis。
- `port_checker`：检查本机或指定地址端口是否可连通。
- `safe_shell`：只执行极少数白名单命令。

## 语义映射

`agent-go/internal/tools/semantic.go` 为每个工具生成安全语义：

- `os_info`：读取公开系统信息，`boundary_level=public`。
- `port_checker`：检查本地网络端口，`boundary_level=low`。
- `service_status`：读取 systemd 服务状态，`resource_type=system_service`，`boundary_level=low`。
- `log_reader`：读取白名单系统日志，`resource_type=system_log`，`boundary_level=sensitive_system_resource`。
- `ssh_login_analyzer`：分析 SSH 认证日志，`operation_type=analyze`，`resource_type=ssh_auth_log`，`boundary_level=sensitive_system_resource`。
- `safe_shell`：白名单命令标记为 `allowed_by_policy=true`；危险命令标记为 `boundary_level=dangerous` 且 `allowed_by_policy=false`。

## Rule-based Ops Planner

`agent-go/internal/agent/planner.go` 当前提供规则规划器，支持：

- `ssh_anomaly_check`：`os_info -> service_status(sshd) -> port_checker(22) -> log_reader(/var/log/secure,/var/log/auth.log) -> ssh_login_analyzer`。
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

## SSH diagnosis

`ssh_login_analyzer` 使用 `auth_log_collector`，来源顺序为：

1. `/var/log/secure`
2. `/var/log/auth.log`
3. `journalctl -u sshd -n 200 --no-pager`

当前识别：

- `Failed password`
- `Failed password for invalid user`
- `Invalid user`
- `Accepted password`
- `Accepted publickey`
- `authentication failure`

风险等级：

- `high`：同一 IP 失败次数大于等于 10。
- `medium`：同一 IP 失败次数大于等于 5，或无效用户尝试大于等于 5。
- `low`：无明显暴力破解模式或只有少量失败。
- `unknown`：认证日志不可用。

该工具只读分析，不自动封禁 IP，不修改防火墙，不删除或移动日志。

## Security Report

`agent-go/internal/report` 将现有运行结果转换为面向展示的报告结构：

- 从 `tool_trace` 生成 `evidence_chain`。
- 从敏感资源边界生成 `sensitive_resources`。
- 从 planner、diagnosis、audit_result 和 sensitive resources 生成 `risk_explanation`。
- 从 diagnosis 和拦截结果生成人工 `recommendations`。

`security_report` 不负责最终裁决，不覆盖 `audit_result`，不改变响应中的 `decision`。

## safe_shell 白名单

- `uname -a`
- `hostname`
- `whoami`
- `date`
- `df -h`
- `free -h`
- `systemctl --version`

`safe_shell` 不使用 shell 管道执行命令，不允许 `rm`、`shutdown`、`reboot`、`mkfs`、`dd`、`chmod 777`、`curl | sh` 等危险模式。
