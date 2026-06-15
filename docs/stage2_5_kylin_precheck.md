# Stage 2.5 Kylin Precheck

Stage 2.5 的目标是加固 Linux/麒麟部署脚本和 E2E 测试脚本，为银河麒麟高级服务器版 V11 x86_64 虚拟机和 LoongArch 环境验证做准备。本阶段不接入 Eino、不做前端、不修改 TraceShield-Core，也不改变 TraceShield 算法语义。

## Windows 已验证内容

- Go Agent -> audit-core-py -> TraceShield 主链路可用。
- `intent_guard` 能在工具执行前短路危险意图。
- safe task 会进入工具执行和 TraceShield 审计。
- `tool_trace` 已携带 Stage 2 语义字段。
- `risk_graph.nodes` 已包含语义节点。

## Linux/麒麟需要验证的差异

- 路径：推荐 `KYLINGUARD_HOME=/opt/kylin-guard-agent`，`TRACESHIELD_CORE_PATH=/opt/traceshield-core`。
- systemd：需要验证 `systemctl`、服务状态读取和后续 service 文件。
- journalctl：需要确认命令是否存在以及普通用户权限。
- `/var/log/*`：需要确认 `/var/log/secure`、`/var/log/auth.log`、`/var/log/audit/audit.log`、`/var/log/messages`、`/var/log/syslog` 的实际路径和权限。
- 权限：日志读取、服务状态读取和端口检查在普通用户与特权用户下行为不同。
- 架构：x86_64 VM 只能做预适配，LoongArch 仍需最终验证。

## 推荐环境变量

```bash
export KYLINGUARD_HOME=/opt/kylin-guard-agent
export TRACESHIELD_CORE_PATH=/opt/traceshield-core
export AUDIT_CORE_URL=http://127.0.0.1:8001
export AGENT_GO_PORT=8080
export AUDIT_CORE_PORT=8001
```

## 运行命令示例

```bash
cd "$KYLINGUARD_HOME"

bash deploy/kylin/check_env.sh
bash deploy/kylin/install_audit_core_py.sh
bash deploy/kylin/install_agent_go.sh

bash deploy/kylin/run_audit_core_py.sh
bash deploy/kylin/run_agent_go.sh

bash scripts/linux/test_agent_e2e.sh
```

## 当前边界

`/api/agent/run` 仍使用静态 planner，暂时不能直接触发 `/var/log/secure` 的 `log_reader` 真实路径读取。Linux E2E 脚本会标记该项为 TODO，后续等 planner/tool selection 支持日志读取任务后再开启硬断言。
