# TODO

## Agent

- 确认 Eino 官方 module path 和 API。
- 用 build tag 或替换 adapter 实现接入真实 Eino planner/runtime。
- 为 `/api/agent/run-eino` 增加真实 Eino 编排路径并保持 intent_guard/audit-core-py 不可绕过。
- 扩展 Rule-based Ops Planner 的场景覆盖，例如磁盘容量、CPU/内存、进程异常、网络连接、审计日志异常。
- 为 planner 增加更严格的服务名、端口和日志意图解析测试集。
- Stage 8 已实现 MCP-like Tool Registry、ToolMetadata、Tool Policy 和 `/api/tools*` 接口。
- 后续可扩展动态插件加载、工具 marketplace 和更细的 schema 校验。
- 扩展 SSH 登录异常诊断的日志格式样例、时间窗口分析、用户名维度和 IP 维度统计。
- 将 `security_report` 支持导出为 Markdown/HTML/PDF 等报告文件。
- 接入远程模型 API provider。
- 增加工具权限策略和审批流。

## Audit Core

- 持续扩展 Kylin 运维工具到 TraceShield tool-name 的适配映射。
- 将 `risk_graph.nodes/edges` 从当前语义展示结构升级为可视化图结构。
- 为 TraceShield fallback 场景增加可观测日志和告警。
- 明确生产环境 `TRACESHIELD_CORE_PATH` 管理方式。
- 将更多 TraceShield 原生 evidence 信息映射为用户可解释证据链。
- 将 TraceShield 原生 violations/evidence 与 `security_report.evidence_chain` 做更细粒度关联。

## Kylin

- 在银河麒麟高级服务器版 V11 上验证 Stage 8 `/api/tools`、`/api/tools/call` 与原 SSH anomaly 链路。
- 验证 LoongArch 构建与运行。
- 补充 systemd service 文件。
- 验证 TraceShield-Core 在 LoongArch Python 环境中的 `pydantic` 和 `PyYAML` 安装。
- 在 Kylin V11 VM 上运行 `deploy/kylin/check_env.sh` 和 `scripts/linux/test_agent_e2e.sh`。
- 验证 `/var/log/*` 读取权限和日志路径差异。
- 验证 `ss`、`netstat`、`journalctl` 在目标系统上的可用性。
- 验证 `journalctl -u sshd` 在 Kylin V11 上的服务名差异。

## Frontend

- Stage 7 已实现单页 Agent 控制台。
- Stage 8 不做前端大改，前端暂时冻结为展示层。
- 后续可增加 risk graph 可视化。
- 后续可增加 security_report Markdown/HTML/PDF 导出。
- 后续可增加 Kylin VM 演示截图和部署说明。

## Tests

- Stage 8 已补充 ToolMetadata、Tool Policy、tools API handler 和 planner metadata 测试。
- 增加更多 Go 单元测试和 HTTP handler 测试，覆盖 planner edge cases。
- 增加更多 SSH 认证日志样例测试，覆盖 Kylin/OpenSSH 常见格式。
- 扩展 Python FastAPI endpoint 测试，覆盖 risky samples 和 fallback 行为。
- 增加 Linux E2E 脚本在 Kylin V11 上的实机验证记录。
- 增加 report builder 的更多场景测试，例如 service_check、port_check 和 fallback mock。
- 增加最小 CI。

## Current Fallback Note

Stage 1 已接入真实 TraceShield 核心入口，并在当前 Windows 环境实测 import 与最小审计调用成功。fallback mock 仍保留，用于 `TRACESHIELD_CORE_PATH` 缺失、TraceShield import 失败或运行时异常时维持 HTTP API 稳定。
