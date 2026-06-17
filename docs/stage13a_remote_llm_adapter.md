# Stage 13A: Controlled Remote LLM Adapter with Structured Tool Plan

## 1. 目标

在不破坏 deterministic-stub 默认行为的前提下，新增真正可配置的 Remote LLM Adapter，使 KylinGuard 可以通过 OpenAI-compatible / DeepSeek-compatible API 接入真实 LLM，用于结构化工具计划生成。

## 2. 配置项

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `EINO_LLM_ENABLED` | `false` | 启用远程 LLM |
| `EINO_LLM_PROVIDER` | `deterministic` | provider 名称 ("deterministic", "openai", "deepseek") |
| `EINO_LLM_ENDPOINT` | 空 | API base URL |
| `EINO_LLM_MODEL` | 空 | 模型名称 |
| `EINO_LLM_API_KEY` | 空 | API key |

默认安全策略：没有 API_KEY 时不会初始化 RemoteLLMAdapter。

## 3. RemoteLLMAdapter 架构

```
RemoteLLMAdapter.GenerateToolCalls(task, toolDefs)
  ├── API key check → 无 key 时返回 error
  ├── HTTP POST → {base}/v1/chat/completions
  │   ├── model, messages(system+user), temperature=0.1
  │   ├── response_format: {type: "json_object"}
  │   └── Authorization: Bearer <API_KEY>
  ├── Parse OpenAI-compatible response
  │   ├── 非 JSON 输出 → error
  │   └── JSON → ParseToolPlanJSON()
  ├── ValidateToolPlan()
  │   ├── scenario/tool_plan 非空
  │   ├── 每个 tool_name 必须在 Tool Registry 中
  │   ├── 禁止 shell/bash/sudo/rm/iptables 等危险命令
  │   └── 禁止 safe_shell
  ├── 有效 → 返回 ToolCalls + Plan
  └── 无效 → 返回 error → Eino runtime fallback to deterministic-stub
```

## 4. OpenAI-compatible 请求格式

```json
POST /v1/chat/completions
Content-Type: application/json
Authorization: Bearer <API_KEY>

{
  "model": "gpt-4",
  "messages": [
    {"role": "system", "content": "You are a security operations assistant..."},
    {"role": "user", "content": "check system resource usage"}
  ],
  "temperature": 0.1,
  "response_format": {"type": "json_object"}
}
```

## 5. Structured Tool Plan Schema

```json
{
  "scenario": "string — e.g. system_resource_check, ssh_anomaly_check",
  "intent": "string — brief intent",
  "tool_plan": [
    {
      "tool_name": "string — must be from allowed tool list",
      "reason": "string",
      "arguments": {}
    }
  ],
  "risk_hint": "low|medium|high",
  "requires_review": false,
  "user_explanation": "string — brief explanation"
}
```

## 6. 安全约束

- LLM 不能直接执行工具
- LLM 不能直接生成 shell 命令
- LLM 不能绕过 intent_guard / Tool Policy / Exec Proxy / TraceShield / decision normalizer
- safe_shell 不允许由 LLM 计划
- shell/bash/sudo/rm/iptables 等命令意图被 `isDangerousToolPlanIntent` 拒绝
- unknown_tool 导致整体 fallback
- API_KEY 不进入 reasoning_trace / 日志 / 前端
- 默认真实 LLM 必须关闭

## 7. Fallback 机制

`FallbackChatModelAdapter` 包装主 adapter，当主 adapter 失败时自动降级到 `DeterministicChatModelStub`：

```
primary → 成功 → 使用 LLM 输出的 tool plan
primary → 失败 → fallback → 使用 deterministic-stub 的关键词匹配
```

fallback 信息记录在 reasoning_trace 的 chat_model span 中：
- `remote_llm_used`: true/false
- `fallback_used`: true/false
- `fallback_reason`: "primary adapter failed: ...; falling back to deterministic stub"

## 8. reasoning_trace 新增字段

chat_model span:
- `chat_model_adapter`: "interface-v1"
- `provider`: "openai" / "deterministic"
- `model`: "gpt-4" 或空
- `llm_enabled`: true/false
- `remote_llm_used`: true/false
- `fallback_used`: true/false
- `fallback_reason`: 字符串
- `output_type`: "structured_tool_plan"
- `tool_count`: 工具数量

## 9. 验证方法

默认配置（`EINO_LLM_ENABLED=false`）下运行：

```bash
cd agent-go && go test ./...
bash -n scripts/linux/test_agent_e2e.sh
```

必须保持：
- Stage 10 checks passed
- Stage 11 checks passed
- Stage 12B checks passed
- Linux/Kylin E2E passed

手动启用真实 LLM（示例，非默认必跑）：

```bash
export EINO_LLM_ENABLED=true
export EINO_LLM_PROVIDER=openai
export EINO_LLM_ENDPOINT=https://api.openai.com/v1
export EINO_LLM_MODEL=gpt-4
# IMPORTANT: fill in your actual API key in a secure way
export EINO_LLM_API_KEY=sk-xxxx
```
