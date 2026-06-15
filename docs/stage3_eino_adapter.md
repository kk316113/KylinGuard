# Stage 3 Eino Adapter

Stage 3 的目标是在不破坏稳定链路的前提下，为后续真实 Eino 编排、多工具选择和 MCP 风格插件化建立 adapter 边界。

## 为什么不直接重写为 Eino

当前稳定主链路已经在 Windows 和银河麒麟 V11 x86_64 上跑通：

```text
/api/agent/run
-> intent_guard
-> static planner/tools
-> semantic trace
-> audit-core-py
-> TraceShield
```

直接重写会增加回归风险，尤其是 intent_guard 短路、TraceShield 审计、Linux/LoongArch 构建和部署脚本稳定性。

## 当前接入方式

- `AgentAdapter` 定义统一运行接口。
- `StableRuntimeAdapter` 包装现有稳定 runtime。
- `EinoAdapter` 是默认禁用的骨架，不 import Eino 外部包。
- `/api/agent/run` 继续使用稳定主链路。
- `/api/agent/run-eino` 是实验链路。

## 默认配置

`EINO_ENABLED=false`

当前即使设置 `EINO_ENABLED=true`，真实 Eino runtime 仍未实现，handler 会 fallback 到 `StableRuntimeAdapter`，并在 `summary` 中标记 `eino real runtime not implemented, stable runtime fallback used`。

## 安全边界

`run-eino` 不允许绕过：

- `intent_guard`
- Tool Registry
- semantic trace
- audit-core-py
- TraceShield adapter

当前 fallback 路径复用稳定 runtime，因此 dangerous task 仍会在工具执行前 deny，safe task 仍会调用 TraceShield。

## 后续替换策略

确认 Eino 包路径和 Kylin 构建方式后，可通过以下方式接入真实实现：

- 使用 build tag 提供 Eino 版本的 adapter 实现。
- 保留默认无外部依赖 adapter，避免 LoongArch 或离线环境构建失败。
- 在真实 Eino adapter 内继续调用现有 security、tools、auditclient 边界。

## Kylin/LoongArch 风险控制

- 不把不确定 Eino import 写入默认构建。
- 不引入重模型依赖。
- 不在本机或 Kylin VM 内运行大模型。
- 每次接入前后都运行 `go test ./...` 和 Linux/Windows E2E。
