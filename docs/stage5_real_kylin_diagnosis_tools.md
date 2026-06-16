# Stage 5: Real Kylin Security Diagnosis Tools

## 目标

Stage 5 将 Stage 4 的“能选择工具”升级为“能做结构化 SSH 登录异常诊断”。当用户请求“检查当前系统 SSH 登录异常”时，Agent 会检查系统与 sshd 状态、检查 22 端口、读取认证日志，并调用 `ssh_login_analyzer` 输出诊断结果。

## 为什么引入 ssh_login_analyzer

`log_reader` 只负责读取白名单日志，不能解释 SSH 登录失败、无效用户、成功登录和来源 IP。`ssh_login_analyzer` 将日志采集和分析封装成独立工具，让诊断行为本身也进入 semantic tool trace，并继续交给 audit-core-py / TraceShield 审计。

## 日志来源策略

`auth_log_collector` 按顺序尝试：

1. `/var/log/secure`
2. `/var/log/auth.log`
3. `journalctl -u sshd -n 200 --no-pager`

文件读取只允许白名单路径：

- `/var/log/secure`
- `/var/log/auth.log`
- `/var/log/messages`
- `/var/log/syslog`
- `/var/log/audit/audit.log`

默认读取最近 `200` 行，最大 `500` 行。Windows 或日志不可用时返回 graceful error，不会导致 Agent 崩溃。

## SSH 登录异常分析规则

当前识别：

- `Failed password`
- `Failed password for invalid user`
- `Invalid user`
- `Accepted password`
- `Accepted publickey`
- `authentication failure`

来源 IP 优先从 `from <ip>` 中提取，支持 IPv4，并简单兼容 IPv6。失败来源 IP 会按次数降序聚合，最多返回 5 个。

## 风险等级

- `high`：同一 IP 失败次数大于等于 10。
- `medium`：同一 IP 失败次数大于等于 5，或无效用户尝试大于等于 5。
- `low`：没有明显暴力破解模式，或只有少量失败。
- `unknown`：没有可用认证日志。

## 不自动封禁 IP

Stage 5 只做只读诊断，不执行任何处置动作。Agent 不会封禁 IP、修改防火墙、停止服务、清空日志或移动日志。后续如果加入处置能力，也必须经过权限策略、intent guard、审计和人工确认。

## diagnosis 与 audit_result

`diagnosis` 是 Agent 的结构化诊断输出，说明日志中观察到的 SSH 登录风险。

`audit_result` 是 audit-core-py / TraceShield 对工具调用链的安全审计输出。

二者不互相覆盖。最终 `decision` 仍来自现有流程和 TraceShield 审计，不由 `diagnosis` 单独决定。

## 与 TraceShield 的关系

`ssh_login_analyzer` 作为普通工具进入 `tool_trace`：

```text
tool_name=ssh_login_analyzer
operation_type=analyze
resource_type=ssh_auth_log
boundary_level=sensitive_system_resource
```

audit-core-py 保持兼容，将该工具映射为 TraceShield 可处理的只读分析类工具，同时 `risk_graph.nodes` 仍展示 KylinGuard 原始语义字段。

## Windows/Kylin 差异

Windows 本机通常没有 `/var/log/secure`、`/var/log/auth.log` 或 `journalctl`，因此 `diagnosis.risk_level` 可能为 `unknown`。

银河麒麟 V11 上如果 `/var/log/secure` 或 `journalctl -u sshd` 可用，`diagnosis` 会尽量输出 `low`、`medium` 或 `high`。

## 当前限制

- 日志解析规则是第一版，覆盖常见 sshd 文本格式。
- 没有读取压缩历史日志。
- 没有跨时间窗口做速率分析。
- 没有自动处置或封禁。
- 仍需在 LoongArch 环境验证。

## 下一步

- 增加更多 Linux 发行版和 Kylin 日志格式样例。
- 增加时间窗口、用户名维度和端口维度分析。
- 将 diagnosis 渲染为最终用户报告。
- 后续让 Eino/LLM planner 选择诊断工具，但必须保留当前安全边界。
