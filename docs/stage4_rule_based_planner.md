# Stage 4: Rule-based Ops Planner

## 为什么引入

Stage 3 之前，`/api/agent/run` 已经具备 intent guard、工具 trace、audit-core-py 和 TraceShield 主链路，但工具选择仍偏静态。用户输入“检查当前系统 SSH 登录异常”时，固定执行 `os_info + port_checker(8080)` 不能体现真实运维诊断路径。

Stage 4 引入 Rule-based Ops Planner，用轻量规则把用户任务映射为可解释 Plan，再按 Plan 顺序调用 Kylin 运维工具。这样可以在不接入真实 LLM、不引入重依赖的前提下，让 trace 更接近真实安全运维过程。

## 当前支持场景

- `ssh_anomaly_check`
  - 触发：SSH 登录异常、登录失败、暴力破解、异常登录、远程登录、failed ssh login、brute force 等。
  - 工具链：`os_info -> service_status(sshd) -> port_checker(22) -> log_reader(/var/log/secure, /var/log/auth.log)`。

- `service_check`
  - 触发：服务状态、检查服务、systemd、sshd、nginx、docker、service status、check service。
  - 工具链：`os_info -> service_status(service_name)`。
  - 当前支持从任务中提取 `sshd`、`nginx`、`docker`，默认 `sshd`。

- `port_check`
  - 触发：端口、监听、开放端口、port、listen、open port。
  - 工具链：`os_info -> port_checker(host=127.0.0.1, port=<extracted>)`。
  - 如果任务中没有 1 到 65535 的端口号，默认 `8080`。

- `system_overview`
  - 默认场景。
  - 工具链：`os_info -> port_checker(8080)`。

## SSH 异常登录检查

“检查当前系统 SSH 登录异常”会生成如下 Plan：

```text
plan-001 os_info
plan-002 service_status service_name=sshd
plan-003 port_checker host=127.0.0.1 port=22
plan-004 log_reader paths=/var/log/secure,/var/log/auth.log lines=100
```

`log_reader` 会按路径顺序尝试读取白名单日志。如果路径不存在或权限不足，工具返回 `status=error` trace，`output_summary` 会说明原因，Agent 不会崩溃，仍会把 error trace 送入 audit-core-py 和 TraceShield 审计。

## 与 intent_guard 的关系

执行顺序保持为：

```text
task
-> intent_guard
-> Rule-based Ops Planner
-> Tool Registry
-> audit-core-py / TraceShield
```

危险任务仍在 planner 之前短路，例如删除审计日志、清空系统日志、格式化磁盘等任务不会生成 plan，不会执行工具，也不会调用 audit-core-py。

## 与 TraceShield 的关系

Planner 只决定工具计划，不改变 TraceShield 算法语义。工具执行后生成的 semantic tool trace 仍通过 Go `auditclient` 发送到 `audit-core-py`，由 TraceShield adapter 转换为审计输入。

Stage 4 增强的是 trace 的运维语义质量，例如：

- `system_service`
- `network_port`
- `system_log`
- `sensitive_system_resource`

## 为什么当前不直接接 LLM

当前比赛工程优先保证可复现、低依赖、跨平台和 Kylin/LoongArch 可部署。真实 LLM planner 会带来 API key、稳定性、提示词注入和评测不可复现问题，因此 Stage 4 先用规则 planner 形成可测基线。

## 后续替换路径

后续可以在不破坏主链路的前提下替换 planner：

```text
Planner interface
-> RuleBasedPlanner
-> EinoPlanner / LLMPlanner
```

真实 Eino 或 LLM planner 必须继续满足：

- 不绕过 intent_guard。
- 不绕过 Tool Registry。
- 不绕过 audit-core-py / TraceShield。
- 输出同样结构的 Plan 和 semantic tool trace。

## 当前限制

- 规则匹配较简单，无法理解复杂多轮上下文。
- 服务名和端口提取只覆盖常见模式。
- 日志路径依赖具体 Linux/麒麟发行版和权限配置。
- Windows 本机通常无法读取 `/var/log/*`，因此 log_reader 会产生 graceful error trace。
