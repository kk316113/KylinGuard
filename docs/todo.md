# TODO

## Agent

- 确认 Eino 官方 module path 和 API。
- 新增 Eino planner/runtime adapter。
- 接入远程模型 API provider。
- 增加工具权限策略和审批流。

## Audit Core

- 根据 Kylin 运维工具扩展 TraceShield tool-name 适配映射。
- 将 `risk_graph.nodes/edges` 从当前 HTTP 兼容结构升级为真实可视化图结构。
- 为 TraceShield fallback 场景增加可观测日志和告警。
- 明确生产环境 `TRACESHIELD_CORE_PATH` 管理方式。

## Kylin

- 在银河麒麟高级服务器版 V11 上验证部署脚本。
- 验证 LoongArch 构建与运行。
- 补充 systemd service 文件。
- 验证 TraceShield-Core 在 LoongArch Python 环境中的 `pydantic` 和 `PyYAML` 安装。

## Frontend

- 设计 Agent 控制台。
- 展示任务、trace、审计结果和最终报告。

## Tests

- 增加 Go 单元测试和 HTTP handler 测试。
- 扩展 Python FastAPI endpoint 测试，覆盖 risky samples 和 fallback 行为。
- 增加最小 CI。

## Current Fallback Note

Stage 1 已接入真实 TraceShield 核心入口，并在当前 Windows 环境实测 import 与最小审计调用成功。fallback mock 仍保留，用于 `TRACESHIELD_CORE_PATH` 缺失、TraceShield import 失败或运行时异常时维持 HTTP API 稳定。
