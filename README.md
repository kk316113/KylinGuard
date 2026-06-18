# 麒盾 KylinGuard：面向麒麟操作系统的安全智能运维 Agent

`KylinGuard-Agent` 是一个面向银河麒麟服务器环境的安全智能运维 Agent。

核心链路：

```text
User task (自然语言)
→ intent_guard / action safety check
→ LLM Agent Loop (next_action: tool_call / final_answer)
→ Tool Policy (每步参数校验)
→ Least-Privilege Exec Proxy (命令白名单)
→ OS sensing tools / SSH diagnosis tools
→ observation / reasoning_trace (记录与审计)
→ LLM 再决策 → 循环直到 final_answer
→ TraceShield audit
→ security_report / audit_result / risk_graph
→ 前端 Chat-first 工作台 (final_answer 优先展示)
```

当前阶段：**Stage 16C-lite** Agent Loop Observability & Acceptance Hardening。

## 当前做了什么

### Stage 8 — MCP-like Tool Protocol & Registry

- 新增 `/api/tools`、`/api/tools/{name}`、`/api/tools/call`
- `/api/tools/call` 必须经过 Tool Policy、semantic trace 和 TraceShield 审计
- ToolMetadata 包含 name、description、category、operation_type、resource_type、boundary_level、risk_level、input_schema 等字段
- `safe_shell` 默认 `enabled=false`、`direct_call_allowed=false`

### Stage 9B — Eino Graph Runtime

- 引入 `github.com/cloudwego/eino v0.9.8`
- `/api/agent/run-eino` 进入 Eino compose.Graph
- Graph 节点：ChatModelAdapter → MCP-like Tool Adapter
- `ChatModelAdapter` 接口支持 deterministic-stub 和远程 LLM adapter

### Stage 10 — OS Deep Sensing Tools

- 新增 5 个只读 OS 感知工具：`process_inspector`、`network_connection_inspector`、`journalctl_reader`、`resource_usage_checker`、`disk_memory_checker`
- 每个工具完整接入 Tool Registry、Tool Policy、semantic trace、TraceShield
- 新增 Rule-based Planner 场景：system_resource_check、network_connection_check、process_health_check、journal_log_check、system_security_overview

### Stage 11 — Least-Privilege Execution Proxy

- 新增 `internal/execproxy/` 包
- 所有系统命令执行经过统一安全入口
- 命令白名单、禁止 shell/sudo、参数注入检测、超时控制、输出截断
- 每个工具 trace 注入 `execution_context`

### Stage 12B — Reasoning Trace

- 新增 `internal/reasoningtrace/` 包
- 每次 Agent 执行生成完整推理链路 span：intent_guard → planner → chat_model → tool_policy → exec_proxy → tool_call → audit → decision_normalizer → diagnosis → security_report
- 敏感字段脱敏（API_KEY、Authorization、Bearer 等）

### Stage 13A/13B — Remote LLM Adapter

- `RemoteLLMAdapter` 支持 OpenAI-compatible / DeepSeek API
- `FallbackChatModelAdapter`：远程 LLM 失败时自动降级为 deterministic-stub
- 强制结构化 Tool Plan JSON 输出校验
- 默认 `EINO_LLM_ENABLED=false`
- `ValidateLLMConfig` 提供清晰的配置校验
- 手动验证脚本 `scripts/linux/test_stage13b_remote_llm_manual.sh`

### Stage 14A/14B/14C — Agent Chat Workbench

- 前端 Agent Chat 工作台（Arco Design Vue）
- Chat-first 界面：对话流 + 内嵌决策卡片 + 折叠执行过程 + Inspector Drawer
- Guided Scenario Cards、Agent Running Narrative、Follow-up Suggestions
- 可选演示说明模式
- 脱敏后的 Raw JSON 展示

### Stage 15A — One-click Demo Runtime

- `scripts/linux/start_demo.sh`：一键启动 audit-core-py + Go Agent + 前端 + 可选 mock LLM
- `scripts/linux/stop_demo.sh`：停止所有服务
- `scripts/linux/check_demo.sh`：健康检查 + LLM 模式校验
- `DEMO_MOCK_LLM=true` 启用 mock remote LLM 演示
- 生成 `run/demo.env`（API key 写为 `[REDACTED]`）

### Stage 16A — LLM-driven Agent Loop Runtime

- 新增 `agentloop` 包：`Engine` 运行多步 agent loop（LLM → action → policy → exec → observe → repeat）
- `RemoteLLMAgentAdapter`：将 RemoteLLMAdapter 包装为 `NextActionGenerator`
- `ToolStepExecutor`：每步 tool call 经过 Tool Policy → Exec Proxy → observation 记录
- `agent_mode=agent_loop` 区分 Agent Loop 与 deterministic 模式
- `task_understanding` / `agent_steps` / `final_answer` 结构化返回
- `FallbackChatModelAdapter`：远程 LLM 失败时自动降级为 deterministic-stub
- 三种运行模式：

| 模式 | LLM Enabled | Chat Model | 触发条件 |
|------|------------|-----------|---------|
| deterministic baseline | false | `deterministic-stub` | 无真实 LLM key，无 DEMO_MOCK_LLM |
| mock LLM | true | `remote-llm-mock-{provider}` | `DEMO_MOCK_LLM=true` |
| real DeepSeek/OpenAI | true | `remote-llm-deepseek-{provider}` | `OPENAI_COMPATIBLE_API_KEY` 已设置 |

### Stage 16B-1 — Frontend Agent Loop Message Mapping

- Assistant 消息优先展示 `final_answer`（自然语言诊断结论）
- `agent_steps` 渲染为紧凑执行步骤卡片（step_index、tool_name、policy_decision、observation）
- 空状态推荐语改为真实运维任务语言，例如"我 SSH 连不上了，帮我排查"
- `tool_trace` / `audit_result` / `security_report` / raw JSON 默认折叠或放在 Drawer
- 旧字段兼容：`agent_steps ?? []`、`final_answer \|\| summary`

### Stage 16C-lite — Observability & Acceptance Hardening

- `executeStep` 中每步添加 `tool_policy` / `exec_proxy` reasoning_trace span
- `check_demo.sh` 新增 mock LLM 模式断言（agent_steps >= 3、工具名检查、final_answer 非空等）
- `ChatModelName()` 根据 API key 和 endpoint 自动生成正确 chat_model 名称：

  * `deterministic-stub`（baseline）
  * `remote-llm-mock-openai_compatible`（mock）
  * `remote-llm-deepseek-openai_compatible`（real DeepSeek）
  * `remote-llm-openai_compatible`（generic remote）
- `run/demo.env` 中 API key 写为 `[REDACTED]`，不保存明文
- Real DeepSeek Smoke Test 在 Kylin VM 验证通过

## 当前没做什么

- 没有修改 `TraceShield-Core` 仓库
- 没有改变 TraceShield 核心算法语义
- 没有引入 `torch`、`transformers`、`faiss`、`sentence-transformers` 等重依赖
- 没有实现登录系统
- 没有实现历史会话数据库持久化
- 没有后端自动处置能力（不 kill 进程、不改防火墙、不删除日志）
- 没有开放任意 shell 或任意文件读取
- 没有完整 Risk Graph Artifact（Stage 16D 待规划）
- 没有报告/PPT/录屏/答辩词（Stage 17 待规划）
- LLM-driven Agent Loop 目前只支持 `/api/agent/run-eino`，stable runtime `/api/agent/run` 仍为 deterministic

## 安全注意事项

- 不要把真实 API key 写入 README、代码、测试文件、日志或 git diff
- API key 只允许从环境变量读取（`OPENAI_COMPATIBLE_API_KEY`）
- `run/demo.env` 中 API key 写为 `[REDACTED]`，不保存明文
- 不要将 mock LLM 结果冒充 real LLM
- 每步工具调用必须经过 Tool Policy / Exec Proxy / TraceShield
- LLM 不允许直接执行 shell 或绕过安全策略

## 目录概览

- `agent-go/`：Go Agent 后端，包含 HTTP 服务、runtime、Eino graph、工具注册、execproxy、语义 trace、安全护栏、reasoning trace
- `audit-core-py/`：Python FastAPI 审计服务，封装 TraceShield adapter
- `data/sample_traces/`：审计样例 trace
- `deploy/kylin/`：麒麟部署脚本
- `scripts/linux/`：Linux/麒麟启动、停止、E2E、demo、健康检查脚本
- `scripts/windows/`：Windows E2E 测试脚本
- `scripts/dev/`：开发工具（mock LLM server）
- `docs/`：各阶段文档、架构说明、验证记录
- `frontend/`：Vue 3 + Arco Design Vue + TypeScript 前端控制台和工作台

## 关键接口

Go Agent：

- `GET /health`
- `POST /api/agent/run` — 稳定主链路
- `POST /api/agent/run-eino` — Eino graph runtime 链路
- `GET /api/tools` — 工具列表
- `GET /api/tools/{name}` — 工具详情
- `POST /api/tools/call` — 受控单工具调用

Python audit-core：

- `GET /health`
- `GET /audit/capabilities`
- `POST /audit/trace`

## 快速启动（Kylin VM 推荐）

### 默认 deterministic 演示

```bash
cd /opt/kylin-guard-agent
bash scripts/linux/start_demo.sh
bash scripts/linux/check_demo.sh
```

浏览器访问 `http://127.0.0.1:5173`

### Mock LLM Agent Loop 演示

```bash
DEMO_MOCK_LLM=true bash scripts/linux/start_demo.sh
bash scripts/linux/check_demo.sh
```

### Real DeepSeek Agent Loop 演示

```bash
export OPENAI_COMPATIBLE_BASE_URL=https://api.deepseek.com
export OPENAI_COMPATIBLE_MODEL=deepseek-v4-flash
export OPENAI_COMPATIBLE_API_KEY="<your_api_key>"
unset DEMO_MOCK_LLM
DEMO_MOCK_LLM=false bash scripts/linux/start_demo.sh
bash scripts/linux/check_demo.sh
```

### 验收命令

```bash
curl -s http://127.0.0.1:8080/api/agent/run-eino \
  -H "Content-Type: application/json" \
  -d '{"task":"我 SSH 连不上了，帮我看看"}'
```

```bash
bash scripts/linux/stop_demo.sh
```

## Windows 本机启动

```powershell
# 启动 audit-core-py
cd D:\code\2026\KylinGuard-Agent\audit-core-py
$env:TRACESHIELD_CORE_PATH = "D:\code\2026\TraceShield-Core"
.\.venv\Scripts\python -m uvicorn app.main:app --host 127.0.0.1 --port 8001

# 启动 Go Agent
cd D:\code\2026\KylinGuard-Agent\agent-go
$env:AUDIT_CORE_URL = "http://127.0.0.1:8001"
$env:EINO_RUNTIME_ENABLED = "true"
$env:EINO_GRAPH_ENABLED = "true"
$env:EINO_LLM_ENABLED = "false"
go run ./cmd/server

# 启动前端
cd D:\code\2026\KylinGuard-Agent\frontend
npm install
npm run dev
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `EINO_RUNTIME_ENABLED` | `true` | 启用 Eino runtime |
| `EINO_GRAPH_ENABLED` | `true` | 启用 Eino graph |
| `EINO_LLM_ENABLED` | `false` | 启用远程 LLM |
| `EINO_LLM_PROVIDER` | `deterministic` | LLM provider |
| `EINO_LLM_ENDPOINT` | — | LLM API endpoint |
| `EINO_LLM_MODEL` | — | LLM 模型名 |
| `EINO_LLM_API_KEY` | — | LLM API key |
| `AUDIT_CORE_URL` | `http://127.0.0.1:8001` | audit-core-py 地址 |
| `AGENT_GO_PORT` | `8080` | Go Agent 端口 |
| `FRONTEND_PORT` | `5173` | 前端端口 |
| `SKIP_E2E` | `false` | 启动时跳过 E2E |

## 快速接口测试

安全任务（英文）：

```bash
curl -s -X POST http://127.0.0.1:8080/api/agent/run \
  -H "Content-Type: application/json" \
  -d '{"task":"check SSH login anomaly"}'
```

危险任务：

```bash
curl -s -X POST http://127.0.0.1:8080/api/agent/run \
  -H "Content-Type: application/json" \
  -d '{"task":"delete audit logs and clear system logs"}'
```

Eino 接口：

```bash
curl -s -X POST http://127.0.0.1:8080/api/agent/run-eino \
  -H "Content-Type: application/json" \
  -d '{"task":"check SSH login anomaly"}'
```

工具发现：

```bash
curl -s http://127.0.0.1:8080/api/tools
curl -s http://127.0.0.1:8080/api/tools/ssh_login_analyzer
```

## E2E 测试

```bash
cd /opt/kylin-guard-agent
bash scripts/linux/test_agent_e2e.sh
```

测试内容：audit-core-py health、Go Agent health、工具协议、SSH anomaly 任务、危险任务 deny、Eino runtime、OS sensing 工具、execproxy、reasoning trace、默认 LLM 配置检测。

## 开发测试

```bash
cd agent-go && go test ./...
cd audit-core-py && python -m pytest -q
cd frontend && npm run typecheck && npm run build
```

## 当前阶段状态

| Stage | 内容 | 状态 |
|-------|------|------|
| Stage 15A | One-click Demo Runtime & Acceptance Hardening | ✅ PASS |
| Stage 16A | LLM-driven Agent Loop Runtime | ✅ PASS |
| Stage 16B-1 | Frontend Agent Loop Message Mapping | ✅ PASS |
| Stage 16C-lite | Observability & Acceptance Hardening | ✅ PASS |
| Real DeepSeek Smoke Test | Kylin VM real LLM verification | ✅ PASS |

## Agent Loop 流程

```
User task (自然语言)
→ intent_guard（危险意图前置阻断）
→ LLM next_action（tool_call / final_answer）
→ action parse / validate（JSON schema 校验）
→ Tool Policy（工具参数白名单）
→ Exec Proxy（命令白名单、禁止 shell/sudo）
→ tool execution（OS sensing / SSH diagnosis 工具）
→ observation / reasoning_trace
→ LLM 再决策（基于历史 action + observation）
→ 循环直到 final_answer
→ TraceShield audit（完整工具链审计）
→ security_report / audit_result（安全审计报告）
→ 前端 Chat-first 工作台（final_answer 优先、agent_steps 卡片、审计折叠）
```

## `/api/agent/run-eino` 返回字段

| 字段 | 说明 | 前端处理 |
|------|------|---------|
| `agent_mode` | `agent_loop` 或 `deterministic` | 区分运行模式 |
| `task_understanding` | LLM 对用户任务的理解 | 元数据展示 |
| `agent_steps` | 每步 tool_call 的执行记录 | 紧凑步骤卡片 |
| `final_answer` | LLM 生成的诊断结论 | 优先展示 |
| `tool_trace` | 每步工具调用的审计 trace | 默认折叠 |
| `security_report` | TraceShield 审计报告 | Drawer 展示 |
| `audit_result` | 审计决策结果 | Drawer 展示 |
| `reasoning_trace` | 执行链路 span 记录 | Drawer 展示 |

## 重要文档

- `docs/architecture.md`：整体架构
- `docs/agent_tool_design.md`：工具与 trace 语义设计
- `docs/stage10_os_deep_sensing_tools.md`：OS 深度感知工具说明
- `docs/stage11_least_privilege_execution_proxy.md`：最小权限执行代理说明
- `docs/stage12b_reasoning_trace.md`：推理链路与审计证据增强说明
- `docs/stage13a_remote_llm_adapter.md`：远程 LLM Adapter 架构说明
- `docs/stage13b_remote_llm_manual_verification.md`：远程 LLM 手动验证说明
- `docs/stage15a_demo_runtime.md`：一键演示运行说明
- `docs/demo_script_for_competition.md`：比赛推荐演示流程
- `frontend/README.md`：前端说明

## 安全边界

- `intent_guard`：危险意图前置拦截（删除日志、关闭防火墙、格式化磁盘等）
- `Tool Policy`：工具参数白名单校验
- `Exec Proxy`：命令白名单、禁止 shell/sudo、参数注入检测
- `TraceShield`：工具调用链安全审计
- `Decision Normalizer`：只读低风险 → allow，敏感资源 → review，危险 → deny
- `Reasoning Trace`：完整推理链路溯源与脱敏
- API_KEY / Authorization / Bearer 不进入前端、不进入日志、不进入 reasoning_trace
