# TODO

## Agent

- Stage 8 已实现 MCP-like Tool Registry、ToolMetadata、Tool Policy 和 `/api/tools*` 接口。
- Stage 9A 已实现 deterministic Eino runtime skeleton 和 MCP-like Tool Adapter。
- Stage 9B 已引入 CloudWeGo Eino `compose.Graph`，并用 deterministic ChatModel Stub 执行 tool-calling 编排。
- Stage 12 已定义 `ChatModelAdapter` 接口，支持 `DeterministicChatModelStub` 和 `RemoteLLMMockAdapter`。
- Stage 12 已添加 `EINO_LLM_PROVIDER`、`EINO_LLM_ENDPOINT`、`EINO_LLM_MODEL`、`EINO_LLM_API_KEY` 环境变量配置。
- Stage 12 已添加 `EinoMetadataPanel` 前端组件，展示 Eino 运行时元数据。
- 后续把 deterministic ChatModel Stub 替换为真实 Eino ChatModel / ReAct Agent，但必须保留 intent_guard、Tool Policy、semantic trace 和 TraceShield 审计。
- 后续可实现真正的 RemoteLLMAdapter，对接 OpenAI / Anthropic 等 API。
- 后续可在 `ChatModelAdapter.GenerateToolCalls` 中传入 `toolDefs`，支持 LLM 工具选择。
- 扩展 Rule-based Ops Planner / deterministic stub 的场景覆盖，例如磁盘容量、CPU/内存、进程异常、网络连接、审计日志异常。
- 为 planner 增加更严格的服务名、端口和日志意图解析测试集。
- 后续可扩展动态插件加载、工具 marketplace 和更细的 schema 校验。
- 接入远程模型 API provider 时，禁止在本机或麒麟 VM 内跑大模型。

## Audit Core

- 持续扩展 Kylin 运维工具到 TraceShield tool-name 的适配映射。
- 将 `risk_graph.nodes/edges` 从当前语义展示结构升级为更完整的可视化图结构。
- 为 TraceShield fallback 场景增加可观测日志和告警。
- 明确生产环境 `TRACESHIELD_CORE_PATH` 管理方式。
- 将更多 TraceShield 原生 evidence 信息映射为用户可解释证据链。
- 将 TraceShield 原生 violations/evidence 与 `security_report.evidence_chain` 做更细粒度关联。

## Kylin

- 在银河麒麟高级服务器版 V11 上继续验证 Stage 12 `/api/agent/run-eino`：`eino_graph_enabled=true`、`chat_model=deterministic-stub`、`chat_model_adapter=interface-v1`、`eino_runtime_version=stage12-v1`。
- 验证 LoongArch 构建与运行。
- 补充 systemd service 文件。
- 验证 TraceShield-Core 在 LoongArch Python 环境中的依赖安装。
- 在 Kylin V11 VM 上持续运行 `deploy/kylin/check_env.sh` 和 `scripts/linux/test_agent_e2e.sh`。
- 验证 `/var/log/*` 读取权限和日志路径差异。
- 验证 `ss`、`netstat`、`journalctl` 在目标系统上的可用性。
- 验证 `journalctl -u sshd` 在 Kylin V11 上的服务名差异。

## Frontend

- Stage 7 已实现单页 Agent 控制台。
- Stage 8/9 期间前端只做必要兼容，仍是展示层。
- 后续可增加 risk graph 可视化。
- 后续可增加 security_report Markdown/HTML/PDF 导出。
- 后续可增加 Kylin VM 演示截图和部署说明。

## Tests

- Stage 8 已补充 ToolMetadata、Tool Policy、tools API handler 和 planner metadata 测试。
- Stage 9A 已补充 Eino runtime skeleton、Tool Adapter 和 run-eino handler 测试。
- Stage 9B 已补充 deterministic ChatModel Stub、Eino graph runtime、run-eino handler metadata 和危险任务前置短路测试。
- Stage 12 已补充 ChatModelAdapter 接口测试、RemoteLLMMockAdapter 测试和 metadata marker 测试。
- 已补充 report builder 场景测试：service_check、port_check、system_resource_check、fallback-mock。
- 已补充 SSH 认证日志样例测试：Kylin secure log、auth.log、journalctl、IPv6、多 IP 暴力破解、高低风险、混合 accepted/failed。
- 已补充 Python FastAPI endpoint 测试：capabilities、risky samples（log_delete、privilege_escalation、sensitive_log_read）、backward compat（traces/tool_trace 字段）、malformed JSON、mixed risk steps。
- 增加 Linux E2E 脚本在 Kylin V11 / LoongArch 上的实机验证记录。
- 增加最小 CI。

## Current Fallback Note

Stage 1 已接入真实 TraceShield 核心入口，并在 Windows / Kylin x86_64 预验证中跑通。fallback mock 仍保留，用于 `TRACESHIELD_CORE_PATH` 缺失、TraceShield import 失败或运行时异常时维持 HTTP API 稳定。
