# Stage 12B: Reasoning Trace & Audit Evidence Enhancement

## 1. 目标

实现一个轻量级 reasoning_trace / audit evidence spans 机制，让每次 Agent 执行都能解释完整链路：

1. 为什么选择这些工具
2. 每个工具访问了什么资源
3. 每个资源的边界等级是什么
4. 使用了什么 execution profile
5. 是否经过 Tool Policy
6. 是否经过 Exec Proxy
7. 是否经过 TraceShield audit
8. decision normalizer 为什么给出 allow / review / deny
9. diagnosis 和 security_report 是怎么生成的

## 2. ReasoningTrace Schema

```go
type ReasoningTrace struct {
    TraceID     string          `json:"trace_id"`
    Runtime     string          `json:"runtime"`       // "stable" or "eino"
    TaskHash    string          `json:"task_hash"`     // non-reversible hash
    TaskSummary string          `json:"task_summary"`  // truncated to 120 chars
    StartedAt   time.Time       `json:"started_at"`
    EndedAt     time.Time       `json:"ended_at"`
    DurationMs  int64           `json:"duration_ms"`
    Spans       []ReasoningSpan `json:"spans"`
}

type ReasoningSpan struct {
    SpanID       string            `json:"span_id"`
    ParentSpanID string            `json:"parent_span_id,omitempty"`
    Type         SpanType          `json:"type"`
    Name         string            `json:"name"`
    Status       string            `json:"status"`
    StartedAt    time.Time         `json:"started_at"`
    EndedAt      time.Time         `json:"ended_at"`
    DurationMs   int64             `json:"duration_ms"`
    Attributes   map[string]any    `json:"attributes,omitempty"`
    Events       []ReasoningEvent  `json:"events,omitempty"`
}
```

## 3. Span 类型说明

| Span Type | 说明 | Stable Runtime | Eino Runtime |
|-----------|------|----------------|--------------|
| `request` | 最外层请求 | ✅ | ✅ |
| `intent_guard` | 安全意图校验 | ✅ | ✅ |
| `planner` | 工具序列规划 | ✅ | ✅ |
| `chat_model` | ChatModel 生成工具调用 | ❌ | ✅ |
| `tool_policy` | 工具策略校验 | ✅ | ✅ |
| `exec_proxy` | 最小权限执行代理 | ✅ | ✅ |
| `tool_call` | 工具调用 | ✅ | ✅ |
| `audit` | TraceShield 审计 | ✅ | ✅ |
| `decision_normalizer` | 决策归一化 | ✅ | ✅ |
| `diagnosis` | 诊断 | ✅ | ✅ |
| `security_report` | 安全报告生成 | ✅ | ✅ |

## 4. 脱敏规则

对所有 attribute keys 和 values 执行脱敏：

- keys 中包含 `api_key`, `authorization`, `bearer`, `token`, `password`, `secret`, `credential` 等 → `[REDACTED]`
- values 中包含 `Bearer `, `sk-`, `-----BEGIN` 等 → `[REDACTED]`
- 日志类工具的输出只写摘要（前 200 字符），不写完整内容

## 5. Stable Runtime 接入

在 `internal/agent/runtime.go` 的 `Run` 方法中构建 `TraceBuilder`，记录：

- request → intent_guard → planner → tool_call → audit → decision_normalizer → diagnosis → security_report
- 每个 tool_call 包含 tool_policy 和 exec_proxy 子 span
- 危险任务：request → intent_guard (deny) → 直接返回

## 6. Eino Runtime 接入

在 `internal/eino/runtime.go` 的 `Run` 方法中构建 `TraceBuilder`，记录：

- request → intent_guard → chat_model → planner → tool_call → audit → decision_normalizer → diagnosis → security_report
- chat_model span 包含 provider/model/llm_enabled 属性
- 与 Stable Runtime 相同的 tool_policy / exec_proxy 子 span

## 7. 与 Tool Policy / Exec Proxy / TraceShield 的关系

- Tool Policy 决策记录为 `tool_policy` span
- Exec Proxy 决策记录为 `exec_proxy` span
- TraceShield audit 记录为 `audit` span，含 `audit_method`, `audit_decision`, `risk_score`, `evidence_count`
- Decision Normalizer 记录原因（为什么 allow/review/deny）

## 8. API 兼容性

在 `RunResponse` 中新增 `reasoning_trace` 字段（`omitempty`），不破坏已有字段。

## 9. 测试与验收方法

### 单元测试
- `internal/reasoningtrace/types_test.go` — schema, builder, 脱敏, duration

### E2E 验收
- `/api/agent/run` safe task 返回 reasoning_trace
- `/api/agent/run-eino` safe task 返回 reasoning_trace
- spans 包含预期类型
- dangerous task 也返回 reasoning_trace（仅 request + intent_guard）
- 无敏感数据泄露

### 运行命令
```bash
cd agent-go && go test ./...
cd audit-core-py && python -m pytest -q
```

## 10. 安全边界说明

- API_KEY / Authorization / Bearer 绝不进入 reasoning_trace
- 不记录完整系统日志
- 不记录完整 journalctl 输出
- 不暴露敏感文件内容
- 不记录完整命令字符串中的注入 payload
