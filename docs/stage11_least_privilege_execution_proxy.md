# Stage 11: Least-Privilege Execution Proxy

## 1. 目标

新增一个最小权限执行代理层 (execproxy)，让所有 OS 工具的系统命令执行都经过统一的安全执行入口。

核心原则：Stage 11 是安全加固阶段，不是自动处置阶段。

## 2. 为什么需要最小权限执行代理

KylinGuard Agent 的工具在执行系统命令（ps, ss, journalctl, df, systemctl, uname 等）时，需要确保：

- 命令在白名单内
- 不使用 shell 解释器
- 不使用 sudo/su 提权
- args 不包含 shell 控制字符
- 执行有超时限制
- 输出有大小限制
- 每一次执行都有完整的审计上下文

## 3. Tool Policy 与 Exec Policy 的区别

| 维度 | Tool Policy | Exec Policy |
|------|-------------|-------------|
| 职责 | 工具是否允许调用 | 命令是否允许执行 |
| 校验 | 工具名、参数合法性 | 命令白名单、参数安全性 |
| 时机 | 工具调用前 | 命令执行前 |
| 拒绝 | `audit_result.method=tool_policy` | `audit_result.method=exec_policy` (via executor) |

两者都必须通过，工具才能执行。

## 4. ExecutionProfile

| Profile | 含义 | 示例工具 |
|---------|------|----------|
| `public_read` | 公开系统信息 | os_info |
| `low_read` | 低风险只读 | process_inspector, network_connection_inspector, port_checker, disk_memory_checker, service_status, resource_usage_checker |
| `sensitive_read` | 敏感系统资源只读 | journalctl_reader, log_reader, ssh_login_analyzer |
| `privileged_read` | 特权只读 | 保留 |
| `denied` | 禁止执行 | 所有危险命令 |

## 5. 命令白名单

| 命令 | 平台 | 用途 |
|------|------|------|
| `ps` | Linux | 进程列表 |
| `pgrep` | Linux | 进程查找 |
| `ss` | Linux | 网络连接 |
| `netstat` | Linux/Windows | 网络连接 |
| `journalctl` | Linux | 系统日志 |
| `df` | Linux | 磁盘使用 |
| `free` | Linux | 内存使用 |
| `uptime` | Linux | 系统运行时间 |
| `cat` | Linux | 文件读取（仅 procfs） |
| `uname` | Linux | 系统信息 |
| `hostname` | Linux | 主机名 |
| `whoami` | Linux | 当前用户 |
| `date` | Linux | 系统时间 |
| `systemctl` | Linux | 服务管理（仅读操作） |
| `tasklist` | Windows | 进程列表 |

## 6. 禁止命令

shell: sh, bash, zsh, dash, cmd, powershell, pwsh, csh, tcsh, ksh
提权: sudo, su, pkexec, doas
危险: rm, mv, cp, chmod, chown, kill, pkill, mount, umount, mkfs, dd, iptables, nft, firewall-cmd, reboot, shutdown

## 7. execution_context 字段

每个 tool_trace 新增 `execution_context` 对象：

```json
{
  "executor": "least_privilege_proxy",
  "profile": "sensitive_read",
  "command_name": "journalctl",
  "args_count": 4,
  "timeout_ms": 5000,
  "max_output_bytes": 131072,
  "shell_used": false,
  "sudo_used": false,
  "allowed_by_exec_policy": true,
  "policy_reason": "allowed journalctl -u sshd -n 100 --no-pager invocation under profile sensitive_read",
  "platform": "linux/amd64"
}
```

## 8. 工具迁移状态

| 工具 | 迁移方式 | 执行方法 |
|------|----------|----------|
| `process_inspector` | executor.Execute() | ps/tasklist |
| `network_connection_inspector` | executor.Execute() | ss/netstat |
| `journalctl_reader` | executor.Execute() | journalctl |
| `disk_memory_checker` | executor.Execute() | df + procfs fallback |
| `service_status` | executor.Execute() | systemctl (read-only) |
| `os_info` | executor.Execute() + NativeExecutionContext | uname + Go runtime |
| `auth_log_collector` | executor.Execute() | journalctl fallback |
| `port_checker` | NativeExecutionContext | Go net.Dial |
| `resource_usage_checker` | NativeExecutionContext | Go procfs reads |
| `log_reader` | NativeExecutionContext | Go os.Open |

## 9. 当前限制

- 不连接真实 LLM
- 不做自动处置
- 不修改系统配置
- 不删除日志

## 10. 下一步计划

- 远程 LLM API Adapter 设计（interface/mock）
- 可审计的受控处置动作（人工确认执行）
- 前端 Eino graph metadata 展示
