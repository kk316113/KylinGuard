# TODO

## Agent

- 确认 Eino 官方 module path 和 API。
- 新增 Eino planner/runtime adapter。
- 接入远程模型 API provider。
- 增加工具权限策略和审批流。

## Audit Core

- 将 `audit-core-py` 从 stub 替换为真实审计服务边界。
- 定义 Go Agent 与 Python Audit Core 的正式协议。
- 接入真实 evidence chain 输出。

## Kylin

- 在银河麒麟高级服务器版 V11 上验证部署脚本。
- 验证 LoongArch 构建与运行。
- 补充 systemd service 文件。

## Frontend

- 设计 Agent 控制台。
- 展示任务、trace、审计结果和最终报告。

## Tests

- 增加 Go 单元测试和 HTTP handler 测试。
- 增加 Python FastAPI endpoint 测试。
- 增加最小 CI。
