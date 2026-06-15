# Project Overview

麒盾 KylinGuard 是面向麒麟操作系统的安全智能运维 Agent。Stage 0 的目标是建立清晰、轻量、可启动的工程骨架，为后续接入真实 Agent 编排、远程模型 API 和论文审计核心预留边界。

## Stage 0 范围

- 初始化 Go Agent 服务。
- 初始化 Python audit-core stub。
- 定义工具 trace 数据结构。
- 定义最小安全意图过滤。
- 定义麒麟部署脚本占位。
- 保持跨平台、跨架构实现思路，避免重依赖。

## 非目标

- 不实现论文方法。
- 不接入本地大模型。
- 不引入 torch、transformers、faiss、sentence-transformers 等重依赖。
- 不实现前端。
