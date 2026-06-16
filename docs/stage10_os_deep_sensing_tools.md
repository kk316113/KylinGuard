# Stage 10: OS Deep Sensing Tools and Minimal Frontend Entry

## 1. 目标

将 KylinGuard 从"SSH 安全诊断 Agent"扩展为"具备进程、网络连接、系统日志、资源使用、磁盘内存等多维 OS 状态感知能力的麒麟安全智能运维 Agent"。

Stage 10 是 OS 感知增强阶段，不是自动处置阶段。所有新增工具均为只读操作。

## 2. 为什么 OS 深度感知是 Agent 核心能力

安全智能运维 Agent 不同于传统审计报表系统的关键点在于：

- 实时获取系统运行状态（进程、网络、资源、日志）
- 在最小权限约束下进行安全感知
- 将感知结果纳入 TraceShield 审计链路
- 所有操作可追溯、可审计

OS 深度感知工具使 Agent 能够回答"系统现在是否安全"这一核心问题。

## 3. 新增工具列表

| 工具名 | 分类 | 操作类型 | 资源类型 | 风险级别 | 边界级别 |
|--------|------|----------|----------|----------|----------|
| `process_inspector` | process | inspect | process | low | low |
| `network_connection_inspector` | network | inspect | network_connection | low | low |
| `journalctl_reader` | log | read | journal_log | medium | sensitive_system_resource |
| `resource_usage_checker` | resource | read | system_resource | low | low |
| `disk_memory_checker` | resource | read | disk_memory | low | low |

## 4. 每个工具的详细说明

### 4.1 process_inspector

- **能力**: 查看关键进程状态，支持按进程名过滤
- **输入**: `{name: "sshd", limit: 20}`
- **输出**: `{processes: [{pid, name, user, state, cmd}], count, source}`
- **安全边界**: 只读 ps/tasklist；不允许 kill/pkill/renice；进程名必须通过安全字符校验
- **实现**: Linux 使用 `ps aux`，Windows 使用 `tasklist`，其他 OS 返回 unsupported

### 4.2 network_connection_inspector

- **能力**: 查看监听端口和网络连接状态
- **输入**: `{state: "LISTEN", limit: 100}`
- **输出**: `{connections: [{protocol, state, local_address, peer_address, process}], count, source}`
- **安全边界**: 只读 ss/netstat；不修改网络配置；state 必须在白名单内
- **实现**: Linux 优先使用 `ss -tunlp`，fallback 到 `netstat -tunlp`；Windows 使用 `netstat -ano`

### 4.3 journalctl_reader

- **能力**: 读取 systemd journal 中指定服务的最近日志
- **输入**: `{service_name: "sshd", lines: 100}`
- **输出**: `{service_name, lines: [...], source: "journalctl", status}`
- **安全边界**: service_name 严格字符校验；不允许自定义参数；lines 1-500；非 Linux 返回 unsupported
- **实现**: `journalctl -u <service_name> -n <lines> --no-pager`

### 4.4 resource_usage_checker

- **能力**: 查看 CPU 负载和内存使用情况
- **输入**: `{}`（无参数）
- **输出**: `{loadavg: {one_min, five_min, fifteen_min}, memory: {mem_total_kb, mem_available_kb, mem_available_ratio}, risk_level}`
- **安全边界**: 只读 /proc/loadavg 和 /proc/meminfo；不接受任意参数
- **实现**: 直接读取 procfs，不使用 shell

### 4.5 disk_memory_checker

- **能力**: 查看磁盘使用率和内存概要
- **输入**: `{include_tmpfs: false}`
- **输出**: `{filesystems: [{filesystem, mountpoint, used_percent}], memory: {...}, risk_level}`
- **安全边界**: 只读 df 和 /proc/meminfo；不执行挂载/卸载/格式化
- **实现**: 优先使用 `df -h`，fallback 到 `/proc/mounts`

## 5. Tool Policy

每个新增工具在 `/api/tools/call` 直接调用时必须通过 Tool Policy 校验：

- **process_inspector**: name 必须通过 `SafeProcessName` 校验（仅字母、数字、下划线、短横线、点号）；limit 1-100；拒绝 shell 注入字符
- **network_connection_inspector**: state 必须在白名单内（LISTEN/ESTABLISHED/TIME-WAIT/CLOSE-WAIT/ALL）；limit 1-500；拒绝 shell 注入字符
- **journalctl_reader**: service_name 必须通过 `SafeJournalServiceName` 校验（字母、数字、下划线、短横线、点号、@）；lines 1-500；拒绝 shell 注入字符
- **resource_usage_checker**: 拒绝 shell 注入字符
- **disk_memory_checker**: 拒绝 shell 注入字符

恶意输入示例（全被 deny）:
- `service_name: "sshd; rm -rf /"`
- `name: "sshd; kill -9 1"`
- `state: "LISTEN; iptables -F"`

## 6. MCP-like Tool Registry 接入

所有 5 个工具已注册到 Stage 8 MCP-like Tool Registry：

- `GET /api/tools` 返回 count >= 11
- `GET /api/tools/{name}` 返回 input_schema / output_schema
- `POST /api/tools/call` 可受控调用
- 所有调用生成 semantic trace
- 所有调用返回 audit_result

## 7. Eino Graph Runtime 接入

Deterministic ChatModel Stub 新增了 5 个任务映射：

1. `执行一次系统安全巡检` → 7 个 tool calls（os_info, resource_usage_checker, disk_memory_checker, network_connection_inspector, service_status, process_inspector, journalctl_reader）
2. `检查当前系统资源使用情况` → 3 个 tool calls（os_info, resource_usage_checker, disk_memory_checker）
3. `检查当前系统网络连接` → 2 个 tool calls（network_connection_inspector, port_checker）
4. `检查 sshd 进程状态` → 2 个 tool calls（process_inspector, service_status）
5. `查看 sshd 最近日志` → 1 个 tool call（journalctl_reader）

所有 tool calls 只选择 Tool Registry 中存在的工具；不选择 safe_shell；危险任务由 intent_guard 前置 deny。

## 8. Diagnosis 规则

新增轻量 diagnosis 场景：

### resource_pressure_check
- 来源: resource_usage_checker, disk_memory_checker
- 磁盘使用率 >= 90% → high
- 磁盘使用率 >= 80% → medium
- mem_available_ratio < 0.1 → high
- mem_available_ratio < 0.2 → medium
- 否则 low

### network_exposure_check
- 来源: network_connection_inspector
- 0.0.0.0:22 LISTEN → medium
- 仅 127.0.0.1:22 → low

### process_health_check
- 来源: process_inspector, service_status
- service active 且 process 存在 → low
- process 不存在 → medium

### journal_log_check
- 来源: journalctl_reader
- 成功读取 → low

### system_security_overview
- 综合上述输出，取最高风险级别

## 9. Security Report 增强

新增场景的 report title：
- `KylinGuard System Security Overview Report`
- `KylinGuard System Resource Security Report`
- `KylinGuard Network Connection Security Report`
- `KylinGuard Process Health Security Report`
- `KylinGuard Journal Log Security Report`

Evidence chain 包含所有新增工具的证据项。Sensitive resources 包含 journal_log。Audit metadata 保留 route/runtime/eino_graph_enabled/chat_model 等字段。

## 10. 前端最小入口

在 TaskRunner 组件的示例任务中新增 5 个按钮：
- 检查当前系统资源使用情况
- 检查当前系统网络连接
- 检查 sshd 进程状态
- 查看 sshd 最近日志
- 执行一次系统安全巡检

不修改 UI 布局、不新增页面、不添加图表。

## 11. 当前限制

- 所有工具为只读操作，不支持修改系统状态
- journalctl_reader 需要 Linux + systemd
- resource_usage_checker 和 disk_memory_checker 仅在 Linux 下提供完整功能
- 非 Linux 环境返回 graceful degradation（unsupported/warning）
- 不接真实 LLM
- 不接外部 API

## 12. 下一步计划

- Stage 11: 最小权限执行代理（白名单化的安全配置变更）
- 或：远程 LLM API Adapter 设计（interface/mock，不填 key）
- 或：前端 Eino graph metadata 展示
- 或：LoongArch 构建验证
- 或：systemd service 文件与安装包化
