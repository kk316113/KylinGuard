# TODO

## Agent

- 确认 Eino 官方 module path 和 API。
- 用 build tag 或替换 adapter 实现接入真实 Eino planner/runtime。
- 为 `/api/agent/run-eino` 增加真实 Eino 编排路径并保持 intent_guard/audit-core-py 不可绕过。
- 扩展 Rule-based Ops Planner 的场景覆盖，例如磁盘容量、CPU/内存、进程异常、网络连接、审计日志异常。
- 为 planner 增加更严格的服务名、端口和日志意图解析测试集。
- 接入远程模型 API provider。
- 增加工具权限策略和审批流。

## Audit Core

- 持续扩展 Kylin 运维工具到 TraceShield tool-name 的适配映射。
- 将 `risk_graph.nodes/edges` 从当前语义展示结构升级为可视化图结构。
- 为 TraceShield fallback 场景增加可观测日志和告警。
- 明确生产环境 `TRACESHIELD_CORE_PATH` 管理方式。
- 将更多 TraceShield 原生 evidence 信息映射为用户可解释证据链。

## Kylin

- 在银河麒麟高级服务器版 V11 上重新验证 Stage 4 planner 工具链。
- 验证 LoongArch 构建与运行。
- 补充 systemd service 文件。
- 验证 TraceShield-Core 在 LoongArch Python 环境中的 `pydantic` 和 `PyYAML` 安装。
- 在 Kylin V11 VM 上运行 `deploy/kylin/check_env.sh` 和 `scripts/linux/test_agent_e2e.sh`。
- 验证 `/var/log/*` 读取权限和日志路径差异。
- 验证 `ss`、`netstat`、`journalctl` 在目标系统上的可用性。

## Frontend

- 设计 Agent 控制台。
- 展示任务、trace、审计结果和最终报告。

## Tests

- 增加更多 Go 单元测试和 HTTP handler 测试，覆盖 planner edge cases。
- 扩展 Python FastAPI endpoint 测试，覆盖 risky samples 和 fallback 行为。
- 增加 Linux E2E 脚本在 Kylin V11 上的实机验证记录。
- 增加最小 CI。

## Current Fallback Note

Stage 1 已接入真实 TraceShield 核心入口，并在当前 Windows 环境实测 import 与最小审计调用成功。fallback mock 仍保留，用于 `TRACESHIELD_CORE_PATH` 缺失、TraceShield import 失败或运行时异常时维持 HTTP API 稳定。
