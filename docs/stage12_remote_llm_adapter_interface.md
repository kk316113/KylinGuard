# Stage 12: Remote LLM API Adapter Interface & Frontend Eino Metadata Display

## 1. 目标

定义可扩展的 ChatModelAdapter 接口，支持确定性 ChatModel Stub 和未来远程 LLM API 共用同一协议；同时在前端展示 Eino 运行时元数据。

Stage 12 是接口设计阶段，不是 LLM 集成阶段。不连接真实 LLM API、不填 API key、不修改安全链路。

## 2. ChatModelAdapter 接口设计

新增 `ChatModelAdapter` 接口，替代原有 `ToolCallGenerator`：

```go
type ChatModelAdapter interface {
    GenerateToolCalls(ctx context.Context, task string, toolDefs []tools.ToolMetadata) ([]ToolCall, agent.Plan, error)
    Name() string
    Provider() string
}
```

### 2.1 接口方法说明

| 方法 | 用途 | 参数说明 |
|------|------|----------|
| `GenerateToolCalls` | 根据任务文本和可用工具定义生成工具调用 | `toolDefs` 可为 nil（stub 不使用，远程 LLM 用于工具选择） |
| `Name` | 返回 adapter 标识符 | 例如 `"deterministic-stub"`, `"remote-llm-mock-openai"` |
| `Provider` | 返回 LLM provider 类型 | 例如 `"deterministic"`, `"openai"`, `"anthropic"` |

### 2.2 ChatModelAdapterConfig

```go
type ChatModelAdapterConfig struct {
    Provider    string // "deterministic", "openai", "anthropic"
    Endpoint    string // LLM API endpoint URL
    Model       string // Model name
    APIKey      string // API key (placeholder)
    Timeout     int    // Request timeout in seconds
    AdapterName string // Custom adapter name override
}
```

## 3. 实现类

### 3.1 DeterministicChatModelStub（现有，重构）

- 实现 `ChatModelAdapter` 接口
- `Name()` 返回 `"deterministic-stub"`
- `Provider()` 返回 `"deterministic"`
- `GenerateToolCalls` 使用原有规则匹配逻辑，忽略 `toolDefs` 参数
- 行为完全不变，向后兼容

### 3.2 RemoteLLMMockAdapter（新增，占位符）

- 实现 `ChatModelAdapter` 接口
- `GenerateToolCalls` 返回 `"remote LLM adapter not implemented"` 错误
- 未来可替换为真实 LLM API 调用实现

## 4. 配置管理

新增环境变量：

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `EINO_LLM_PROVIDER` | `"deterministic"` | LLM provider 类型 |
| `EINO_LLM_ENDPOINT` | `""` | LLM API endpoint URL |
| `EINO_LLM_MODEL` | `""` | 模型名称 |
| `EINO_LLM_API_KEY` | `""` | API key（占位符） |

Adapter 选择逻辑：
- `EINO_LLM_ENABLED=true && EINO_LLM_PROVIDER != "deterministic"` → `RemoteLLMMockAdapter`
- 否则 → `DeterministicChatModelStub`（默认）

## 5. 安全边界保留

Stage 12 不修改任何安全链路：

- `intent_guard` 前置危险意图拦截：保留
- `Tool Policy` 工具级别策略校验：保留
- `semantic trace` 语义工具调用链：保留
- `audit-core-py / TraceShield` 审计：保留
- `/api/agent/run` 稳定链路：不受影响
- `safe_shell` 直连禁止：保留

## 6. 前端元数据显示

### 6.1 EinoMetadataPanel 组件

新增 `EinoMetadataPanel.vue`，使用 `el-descriptions` 展示以下字段：

| 字段 | 说明 |
|------|------|
| `runtime` | 运行时名称 |
| `route` | 路由标识 |
| `eino_graph_enabled` | 是否启用 Eino graph |
| `llm_enabled` | 是否启用 LLM |
| `chat_model` | ChatModel 名称 |
| `chat_model_adapter` | Adapter 接口版本 |
| `orchestration` | 编排方式 |
| `tool_protocol` | 工具协议 |
| `eino_runtime_version` | Eino 运行时版本 |
| `tools_used` | 使用的工具列表 |

### 6.2 ReportTabs 集成

- 条件渲染：仅当 `audit_metadata.runtime === 'eino'` 时显示 "Eino Metadata" tab
- 不影响 stable runtime 的报告展示

## 7. security_report.audit_metadata 新增字段

| 字段 | 值 | 说明 |
|------|-----|------|
| `chat_model_adapter` | `"interface-v1"` | Stage 12 adapter 接口版本标记 |

其他已有字段（`route`, `runtime`, `eino_graph_enabled`, `llm_enabled`, `chat_model`, `orchestration`, `tool_protocol`, `eino_runtime_version`）保持不变。

## 8. 文件变更清单

| 操作 | 文件 | 说明 |
|------|------|------|
| 新建 | `agent-go/internal/eino/chat_model_adapter.go` | 接口定义和配置结构体 |
| 新建 | `agent-go/internal/eino/remote_llm_adapter.go` | Remote LLM mock 实现 |
| 修改 | `agent-go/internal/eino/types.go` | 版本号、常量、Config/Metadata 字段 |
| 修改 | `agent-go/internal/eino/deterministic_chat_model.go` | 实现新接口、移除旧 ToolCallGenerator |
| 修改 | `agent-go/internal/eino/graph_runtime.go` | 使用 ChatModelAdapter |
| 修改 | `agent-go/internal/eino/runtime.go` | adapter 选择逻辑、metadata 标记 |
| 修改 | `agent-go/internal/config/config.go` | 新增 LLM 环境变量 |
| 修改 | `agent-go/cmd/server/main.go` | 传递 LLM 配置 |
| 新建 | `frontend/src/components/EinoMetadataPanel.vue` | Eino 元数据展示组件 |
| 修改 | `frontend/src/components/ReportTabs.vue` | 集成 EinoMetadataPanel |

## 9. 当前限制

- 不连接真实 LLM API
- 不填 API key
- 不引入模型 SDK
- RemoteLLMMockAdapter 返回 not-implemented 错误
- 不修改安全链路
- 不修改 `/api/agent/run` 行为

## 10. 下一步计划

- 实现真正的 RemoteLLMAdapter，对接 OpenAI / Anthropic 等 API
- 在 `GenerateToolCalls` 中传入 `toolDefs`，支持 LLM 工具选择
- 可审计的受控处置动作（人工确认执行）
- 前端 risk graph 可视化
- LoongArch 构建验证
- systemd service 文件
