# 麒盾 KylinGuard：面向麒麟操作系统的安全智能运维 Agent

`KylinGuard-Agent` 是一个面向银河麒麟服务器环境的安全智能运维 Agent 比赛工程。当前重点不是跑本地大模型，而是建立一条可验证、可审计、可逐步演进的 Agent 安全链路：

```text
User Task
-> Go Agent Runtime
-> Intent Guard
-> Rule-based Ops Planner
-> Tool Registry
-> Kylin Ops Tools
-> SSH Diagnosis Tools
-> Semantic Tool Trace
-> audit-core-py
-> TraceShield Adapter
-> TraceShield Core
-> Security Report
-> Audit Result / Risk Graph
```

当前阶段：Stage 7：KylinGuard Frontend Security Console Implementation。

## 当前做了什么

- 初始化了 Go Agent 后端服务，默认端口 `8080`。
- 初始化了 Python `audit-core-py` 审计服务，默认端口 `8001`。
- 接入清洗后的 TraceShield 论文审计核心，Go Agent 只通过 HTTP 调用，不直接 import TraceShield。
- 增加了 `intent_guard` 前置安全护栏，危险任务会在工具执行前短路 deny。
- 增加了运维工具注册表和基础工具：`os_info`、`port_checker`、`service_status`、`log_reader`、`safe_shell`。
- 扩展了工具调用 trace，使其携带 `operation_type`、`resource_type`、`boundary_level`、`allowed_by_policy` 等语义字段。
- audit-core-py 会把语义 trace 映射到 `risk_graph.nodes`，便于后续解释和可视化。
- 加入了可选 Eino Adapter 骨架和实验接口 `/api/agent/run-eino`。
- 加入了 Rule-based Ops Planner，可以根据 SSH 异常、服务状态、端口检查、系统概览等任务生成可解释 Plan。
- 增强了 `service_status` 和 `log_reader`，支持受控 systemctl 探测和白名单系统日志读取。
- 新增 `ssh_login_analyzer`，可以采集认证日志或 journalctl，分析 SSH 登录失败、无效用户、成功登录和来源 IP。
- `/api/agent/run` 会在 SSH 异常场景返回 `diagnosis`，但最终安全判定仍由 audit-core-py / TraceShield 给出。
- 新增确定性 `security_report`，将 plan、tool_trace、diagnosis、audit_result 组织为 evidence chain、risk explanation、sensitive resources 和 recommendations。
- 新增 Vue 3 前端控制台，用于触发 Go Agent 任务并展示 plan、diagnosis、tool_trace、security_report 和 Raw JSON。
- 加固了 Linux/麒麟部署脚本和 Windows/Linux E2E 测试脚本。
- 已在 Windows 本机和银河麒麟高级服务器版 V11 x86_64 VM 上完成预验证。

## 当前没做什么

- 没有修改 `TraceShield-Core` 仓库。
- 没有改变 TraceShield 核心算法语义。
- 没有实现 Boundary Lattice、真实 Data-flow Evidence、真实 Provenance Contract。
- 没有接入真实 Eino 运行时，当前只是 adapter 骨架。
- 没有接入真实 LLM 或本地大模型。
- 没有引入 `torch`、`transformers`、`faiss`、`sentence-transformers` 等重依赖。
- 没有实现前端页面。
- LoongArch 环境尚未完成最终验证。
- Rule-based Planner 仍是轻量规则匹配，不等同于真实 LLM/Eino 智能规划。
- `ssh_login_analyzer` 不会自动封禁 IP，不会修改防火墙，也不会删除或移动日志。
- `security_report` 不负责最终裁决，不会覆盖 `decision` 或 `audit_result`。
- 前端不是安全边界，不直接调用 audit-core-py，不执行命令，不提供自动处置按钮。

## 目录概览

- `agent-go/`
  Go Agent 后端。包含 HTTP 服务、稳定 runtime、Eino adapter 骨架、工具注册、语义 trace、安全护栏和 audit-core HTTP client。

- `audit-core-py/`
  Python FastAPI 审计服务。封装 TraceShield adapter，对外提供 `/health`、`/audit/capabilities`、`/audit/trace`。

- `data/sample_traces/`
  审计样例 trace，包括安全系统检查、敏感日志读取、清空日志、提权等样例。

- `deploy/kylin/`
  面向 Linux/银河麒麟的环境检查、安装和启动脚本。

- `scripts/windows/`
  Windows 本机 E2E 测试脚本。

- `scripts/linux/`
  Linux/麒麟 E2E 测试脚本。

- `docs/`
  阶段说明、架构说明、工具语义设计、麒麟适配说明、验证记录和 TODO。

- `frontend/`
  Vue 3 + Vite + TypeScript 前端控制台，展示安全运维任务、执行计划、审计摘要、证据链、敏感资源、风险解释、建议和 Raw JSON。

## 关键接口

Go Agent：

- `GET /health`
- `GET /api/os/info`
- `POST /api/agent/run`
- `POST /api/agent/run-eino`

Python audit-core：

- `GET /health`
- `GET /audit/capabilities`
- `POST /audit/trace`

`/api/agent/run` 是稳定主链路。
`/api/agent/run-eino` 是实验链路；当前默认 fallback 到稳定 runtime，并在 `summary` 中标记 `stable runtime fallback used`。

`/api/agent/run` 和 `/api/agent/run-eino` 当前都会返回可选字段 `plan`。危险任务被 `intent_guard` 短路时不会返回 plan。

SSH 登录异常场景还会返回可选字段 `diagnosis`。`diagnosis` 是诊断结果，不覆盖 `audit_result`。

所有 Agent run 响应当前都会返回可选字段 `security_report`。它是面向展示和报告的解释结构，不改变最终 `decision`。

## 环境变量

Windows 本机常用：

```powershell
$env:TRACESHIELD_CORE_PATH = "D:\code\2026\TraceShield-Core"
$env:AUDIT_CORE_URL = "http://127.0.0.1:8001"
$env:KYLIN_GUARD_AGENT_PORT = "8080"
$env:EINO_ENABLED = "false"
```

Linux/麒麟推荐：

```bash
export KYLINGUARD_HOME=/opt/kylin-guard-agent
export TRACESHIELD_CORE_PATH=/opt/traceshield-core
export AUDIT_CORE_URL=http://127.0.0.1:8001
export AGENT_GO_PORT=8080
export AUDIT_CORE_PORT=8001
export EINO_ENABLED=false
```

## Windows 本机启动

先启动 audit-core-py：

```powershell
cd D:\code\2026\KylinGuard-Agent\audit-core-py
python -m venv .venv
.\.venv\Scripts\python -m pip install -r requirements.txt
$env:TRACESHIELD_CORE_PATH = "D:\code\2026\TraceShield-Core"
.\.venv\Scripts\python -m uvicorn app.main:app --host 127.0.0.1 --port 8001
```

再启动 Go Agent：

```powershell
cd D:\code\2026\KylinGuard-Agent\agent-go
$env:AUDIT_CORE_URL = "http://127.0.0.1:8001"
$env:EINO_ENABLED = "false"
go run ./cmd/server
```

如果 `8080` 被旧进程占用：

```powershell
netstat -ano | findstr :8080
taskkill /PID <PID> /F
```

启动前端控制台：

```powershell
cd D:\code\2026\KylinGuard-Agent\frontend
npm install
npm run dev
```

默认访问：

```text
http://127.0.0.1:5173
```

前端通过 Vite proxy 调用 Go Agent：

- `/health`
- `/api/agent/run`
- `/api/agent/run-eino`

## Linux/麒麟部署启动

在目标机器上建议将仓库放到：

```bash
/opt/kylin-guard-agent
```

TraceShield-Core 建议放到：

```bash
/opt/traceshield-core
```

预检查环境：

```bash
cd /opt/kylin-guard-agent
bash deploy/kylin/check_env.sh
```

安装 Python 审计服务依赖：

```bash
export KYLINGUARD_HOME=/opt/kylin-guard-agent
export TRACESHIELD_CORE_PATH=/opt/traceshield-core
bash deploy/kylin/install_audit_core_py.sh
```

安装/构建 Go Agent：

```bash
export KYLINGUARD_HOME=/opt/kylin-guard-agent
bash deploy/kylin/install_agent_go.sh
```

启动 audit-core-py：

```bash
export KYLINGUARD_HOME=/opt/kylin-guard-agent
export TRACESHIELD_CORE_PATH=/opt/traceshield-core
export AUDIT_CORE_PORT=8001
bash deploy/kylin/run_audit_core_py.sh
```

启动 Go Agent：

```bash
export KYLINGUARD_HOME=/opt/kylin-guard-agent
export AUDIT_CORE_URL=http://127.0.0.1:8001
export AGENT_GO_PORT=8080
export EINO_ENABLED=false
bash deploy/kylin/run_agent_go.sh
```

也可以使用 Linux/麒麟一键启动脚本同时拉起 audit-core-py 和 Go Agent：

```bash
export KYLINGUARD_HOME=/opt/kylin-guard-agent
export TRACESHIELD_CORE_PATH=/opt/traceshield-core
export AUDIT_CORE_PORT=8001
export AGENT_GO_PORT=8080
export AUDIT_CORE_URL=http://127.0.0.1:8001
bash scripts/linux/start_all.sh
```

停止服务：

```bash
export KYLINGUARD_HOME=/opt/kylin-guard-agent
bash scripts/linux/stop_all.sh
```

一键脚本会将日志写入 `logs/audit-core.log` 和 `logs/agent-go.log`，并在 `run/` 目录记录进程 PID。

当前预期支持 `x86_64`、`aarch64`、`loongarch64`。x86_64 麒麟 VM 已完成预验证；LoongArch 仍需最终验证。

## 快速接口测试

健康检查：

```bash
curl http://127.0.0.1:8001/health
curl http://127.0.0.1:8080/health
```

安全任务：

```bash
curl -s -X POST http://127.0.0.1:8080/api/agent/run \
  -H "Content-Type: application/json; charset=utf-8" \
  --data-binary '{"task":"检查当前系统 SSH 登录异常"}'
```

预期：

- `decision=allow` 或 `review`
- `audit_result.method=traceshield`
- `plan.scenario=ssh_anomaly_check`
- `plan.steps` 包含 `os_info`、`service_status`、`port_checker`、`log_reader`、`ssh_login_analyzer`
- `diagnosis.risk_level` 为 `low`、`medium`、`high` 或 `unknown`
- `security_report.overall_decision` 等于当前 `decision`
- `security_report.evidence_chain` 覆盖计划中的工具步骤
- `security_report.risk_explanation` 包含 `planner`、`diagnosis`、`boundary_audit`，访问敏感资源时包含 `sensitive_resource`
- `tool_trace` 非空
- trace 中包含 `operation_type`、`resource_type`、`boundary_level`
- trace 中包含 `system_service`、`network_port`、`system_log`、`ssh_auth_log`

危险任务：

```bash
curl -s -X POST http://127.0.0.1:8080/api/agent/run \
  -H "Content-Type: application/json; charset=utf-8" \
  --data-binary '{"task":"delete audit logs and clear system logs"}'
```

预期：

- `decision=deny`
- `audit_result.method=intent_guard`
- `tool_trace=[]`
- `plan` 为空或不存在
- `diagnosis` 为空或不存在
- `security_report.overall_decision=deny`
- `security_report.risk_explanation` 包含 `dangerous_intent`

Eino 实验接口：

```bash
curl -s -X POST http://127.0.0.1:8080/api/agent/run-eino \
  -H "Content-Type: application/json; charset=utf-8" \
  --data-binary '{"task":"检查当前系统 SSH 登录异常"}'
```

预期：

- 返回结构与 `/api/agent/run` 一致
- `summary` 包含 `stable runtime fallback used`
- 当前仍走稳定 runtime、Rule-based Ops Planner、SSH diagnosis 工具链和 TraceShield 审计
- `security_report.audit_metadata.route=eino-fallback`

## E2E 测试

Windows：

```powershell
cd D:\code\2026\KylinGuard-Agent
.\scripts\windows\test_agent_e2e.ps1
```

Linux/麒麟：

```bash
cd /opt/kylin-guard-agent
bash scripts/linux/test_agent_e2e.sh
```

脚本会测试：

- audit-core-py `/health`
- Go Agent `/health`
- `/api/agent/run` safe task
- `/api/agent/run` dangerous task
- `/api/agent/run-eino` safe task
- `/api/agent/run-eino` dangerous task
- safe task 的 `plan.scenario=ssh_anomaly_check`
- safe task 的 plan steps 和 semantic tool trace
- safe task 的 `diagnosis.risk_level`
- safe/dangerous/run-eino task 的 `security_report`

## 开发测试

Go：

```bash
cd agent-go
gofmt -w .
go test ./...
```

Python：

```bash
cd audit-core-py
python -m pytest -q
```

## TraceShield 接入说明

TraceShield 是清洗后的论文核心方法来源，默认源码路径：

```text
D:\code\2026\TraceShield-Core
```

Linux 推荐路径：

```text
/opt/traceshield-core
```

当前采用策略 A：`audit-core-py` 通过 `TRACESHIELD_CORE_PATH` 动态加入 Python import 路径，并调用 `traceshield_experiment_core.TraceShieldEvaluator`。KylinGuard 不复制整个 TraceShield 仓库，不修改其内部逻辑。

如果 TraceShield 无法 import 或运行失败，audit-core-py 会返回 `method=fallback-mock`，并在 `message` 中说明 fallback 原因。

## 安全边界

`intent_guard` 负责前置危险意图拦截，例如：

- 清空系统日志
- 删除审计记录
- 关闭防火墙
- 格式化磁盘
- delete audit logs
- clear system logs
- rm -rf
- curl | sh

TraceShield 负责工具调用链审计，例如：

- 工具动作是否超出用户目标
- 是否访问敏感系统资源
- 是否出现危险命令
- 证据链和 risk graph 生成

二者共同构成 KylinGuard 的安全护栏。

## 工具语义字段

每个 tool trace 会尽量包含：

- `operation_type`
- `resource_type`
- `resource_path`
- `permission_scope`
- `boundary_level`
- `tool_semantic`
- `requires_privilege`
- `allowed_by_policy`
- `policy_reason`

这些字段用于帮助 audit-core-py 生成更清晰的 `risk_graph.nodes`，也为后续报告和可视化做准备。

## Rule-based Ops Planner

当前支持四类场景：

- `ssh_anomaly_check`：SSH 登录异常、登录失败、暴力破解、异常登录等任务。
- `service_check`：检查 `sshd`、`nginx`、`docker` 等服务状态。
- `port_check`：检查指定端口监听或开放状态。
- `system_overview`：默认系统概览。

例如“检查当前系统 SSH 登录异常”会生成：

```text
plan-001 os_info
plan-002 service_status service_name=sshd
plan-003 port_checker host=127.0.0.1 port=22
plan-004 log_reader paths=/var/log/secure,/var/log/auth.log lines=100
plan-005 ssh_login_analyzer paths=/var/log/secure,/var/log/auth.log lines=200
```

`log_reader` 只允许读取白名单日志路径。如果路径不存在或权限不足，会生成 `status=error` 的 graceful trace，不会导致 Agent 崩溃。

`ssh_login_analyzer` 会按 `/var/log/secure`、`/var/log/auth.log`、`journalctl -u sshd` 的顺序采集认证日志，并分析：

- SSH 登录失败次数
- 无效用户尝试次数
- 成功登录次数
- Top failed source IPs
- `low` / `medium` / `high` / `unknown` 风险等级

## Security Report

`security_report` 由 Go Agent 侧的 deterministic report builder 生成，主要字段包括：

- `title`
- `scenario`
- `overall_decision`
- `risk_level`
- `summary`
- `evidence_chain`
- `risk_explanation`
- `recommendations`
- `sensitive_resources`
- `audit_metadata`

它只解释现有诊断与审计结果，不负责裁决。最终 `decision` 仍来自 intent_guard / TraceShield / 现有 decision flow。

## Eino Adapter 状态

当前没有引入真实 Eino 依赖。

`EINO_ENABLED=false` 时：

- `/api/agent/run-eino` fallback 到稳定 runtime
- 不绕过 intent_guard
- 不绕过 audit-core-py
- 不改变 `/api/agent/run` 行为

未来确认 Eino 包路径、版本和麒麟/LoongArch 构建方式后，再通过 build tag 或替换 adapter 实现真实接入。

## 重要文档

- `docs/architecture.md`：整体架构
- `docs/agent_tool_design.md`：工具与 trace 语义设计
- `docs/stage1_5_validation.md`：intent_guard 短路验证
- `docs/stage2_tool_semantics.md`：工具语义映射说明
- `docs/stage2_5_kylin_precheck.md`：麒麟预检查说明
- `docs/stage3_eino_adapter.md`：Eino Adapter 接入说明
- `docs/stage4_rule_based_planner.md`：Rule-based Ops Planner 说明
- `docs/stage5_real_kylin_diagnosis_tools.md`：真实 SSH 登录异常诊断工具链说明
- `docs/stage6_audit_report_evidence_chain.md`：审计报告与证据链说明
- `docs/todo.md`：后续计划
- `frontend/README.md`：前端控制台启动与安全边界说明

## 后续 TODO

- 在 LoongArch 环境完成最终验证。
- 增加 systemd service 文件。
- 扩展 Rule-based Ops Planner，让更多安全运维任务可以选择真实工具链。
- 扩展 SSH 日志格式、时间窗口和用户名/IP 维度诊断。
- 将 `security_report` 导出为前端页面或报告文件。
- 扩展前端展示，例如 risk graph、报告导出和 Kylin VM 演示截图。
- 将更多 TraceShield evidence 映射为用户可解释报告。
- 接入真实 Eino runtime。
- 接入远程 LLM API。
- 实现前端控制台和报告页面。
