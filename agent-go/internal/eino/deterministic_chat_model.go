package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

type ToolCall struct {
	ID         string          `json:"id"`
	ToolName   string          `json:"tool_name"`
	Input      map[string]any  `json:"input"`
	Reason     string          `json:"reason"`
	SchemaCall schema.ToolCall `json:"schema_tool_call"`
}

type DeterministicChatModelStub struct {
	registry *tools.Registry
	planner  agent.Planner
	guard    security.IntentGuard
}

func NewDeterministicChatModelStub(registry *tools.Registry) *DeterministicChatModelStub {
	if registry == nil {
		registry = tools.NewDefaultRegistry()
	}
	return &DeterministicChatModelStub{
		registry: registry,
		planner:  agent.NewRuleBasedPlannerWithRegistry(registry),
		guard:    security.NewIntentGuard(),
	}
}

// GenerateToolCalls implements ChatModelAdapter.
// The toolDefs parameter is accepted but not used by the deterministic stub.
func (m *DeterministicChatModelStub) GenerateToolCalls(ctx context.Context, task string, toolDefs []tools.ToolMetadata) ([]ToolCall, agent.Plan, error) {
	trimmed := strings.TrimSpace(task)
	if trimmed == "" {
		return nil, agent.Plan{}, fmt.Errorf("task is required")
	}
	if m.guard.Evaluate(trimmed).Decision == security.DecisionDeny {
		return nil, agent.Plan{}, fmt.Errorf("dangerous task must be denied by intent_guard before chat model")
	}

	normalized := normalizeTask(trimmed)
	var calls []ToolCall
	var scenario string
	var summary string

	switch {
	case isDeterministicSSHAnomalyTask(normalized):
		scenario = "ssh_anomaly_check"
		summary = "Deterministic ChatModel Stub selected SSH anomaly tool calls"
		calls = []ToolCall{
			m.toolCall(1, "os_info", map[string]any{}, "Collect OS context before diagnosis"),
			m.toolCall(2, "service_status", map[string]any{"service_name": "sshd"}, "Check sshd service status"),
			m.toolCall(3, "port_checker", map[string]any{"host": "127.0.0.1", "port": 22}, "Check whether local SSH port is reachable"),
			m.toolCall(4, "log_reader", map[string]any{
				"paths":   []string{"/var/log/secure", "/var/log/auth.log"},
				"lines":   100,
				"purpose": "ssh_login_anomaly_check",
			}, "Read recent SSH authentication logs if available"),
			m.toolCall(5, "ssh_login_analyzer", map[string]any{
				"paths": []string{"/var/log/secure", "/var/log/auth.log"},
				"lines": 200,
			}, "Analyze SSH authentication failures, accepted logins, and top failed source IPs"),
		}
	case isDeterministicSystemSecurityOverviewTask(normalized):
		scenario = "system_security_overview"
		summary = "Deterministic ChatModel Stub selected system security overview tool calls"
		calls = []ToolCall{
			m.toolCall(1, "os_info", map[string]any{}, "Collect basic OS context"),
			m.toolCall(2, "resource_usage_checker", map[string]any{}, "Check system load and memory usage"),
			m.toolCall(3, "disk_memory_checker", map[string]any{"include_tmpfs": false}, "Check disk usage and memory summary"),
			m.toolCall(4, "network_connection_inspector", map[string]any{"state": "LISTEN", "limit": 100}, "Check network listening ports"),
			m.toolCall(5, "service_status", map[string]any{"service_name": "sshd"}, "Check sshd service status"),
			m.toolCall(6, "process_inspector", map[string]any{"name": "sshd", "limit": 20}, "Inspect sshd process status"),
			m.toolCall(7, "journalctl_reader", map[string]any{"service_name": "sshd", "lines": 100}, "Read recent sshd journal logs"),
		}
	case isDeterministicSystemResourceCheckTask(normalized):
		scenario = "system_resource_check"
		summary = "Deterministic ChatModel Stub selected system resource check tool calls"
		calls = []ToolCall{
			m.toolCall(1, "os_info", map[string]any{}, "Collect OS context before resource check"),
			m.toolCall(2, "resource_usage_checker", map[string]any{}, "Check system load and memory usage"),
			m.toolCall(3, "disk_memory_checker", map[string]any{"include_tmpfs": false}, "Check disk usage and memory summary"),
		}
	case isDeterministicNetworkConnectionCheckTask(normalized):
		scenario = "network_connection_check"
		summary = "Deterministic ChatModel Stub selected network connection check tool calls"
		calls = []ToolCall{
			m.toolCall(1, "network_connection_inspector", map[string]any{"state": "LISTEN", "limit": 100}, "Inspect listening network ports"),
			m.toolCall(2, "port_checker", map[string]any{"host": "127.0.0.1", "port": 22}, "Check whether local SSH port is reachable"),
		}
	case isDeterministicProcessHealthCheckTask(normalized):
		scenario = "process_health_check"
		summary = "Deterministic ChatModel Stub selected process health check tool calls"
		calls = []ToolCall{
			m.toolCall(1, "process_inspector", map[string]any{"name": "sshd", "limit": 20}, "Inspect sshd process status"),
			m.toolCall(2, "service_status", map[string]any{"service_name": "sshd"}, "Check sshd service status"),
		}
	case isDeterministicJournalLogCheckTask(normalized):
		scenario = "journal_log_check"
		summary = "Deterministic ChatModel Stub selected journal log check tool call"
		calls = []ToolCall{
			m.toolCall(1, "journalctl_reader", map[string]any{"service_name": "sshd", "lines": 100}, "Read recent sshd journal logs"),
		}
	case isDeterministicSSHDServiceTask(normalized):
		scenario = "service_check"
		summary = "Deterministic ChatModel Stub selected sshd service status tool call"
		calls = []ToolCall{
			m.toolCall(1, "service_status", map[string]any{"service_name": "sshd"}, "Check sshd service status"),
		}
	case isDeterministicPort22Task(normalized):
		scenario = "port_check"
		summary = "Deterministic ChatModel Stub selected SSH port inspection tool call"
		calls = []ToolCall{
			m.toolCall(1, "port_checker", map[string]any{"host": "127.0.0.1", "port": 22}, "Check whether local SSH port is reachable"),
		}
	default:
		plan, err := m.planner.Plan(ctx, trimmed)
		if err != nil {
			return nil, agent.Plan{}, err
		}
		calls = callsFromPlan(plan)
		return calls, plan, nil
	}

	plan := planFromToolCalls(trimmed, scenario, summary, calls, m.registry)
	return calls, plan, nil
}

// Name returns the adapter identifier.
func (m *DeterministicChatModelStub) Name() string {
	return "deterministic-stub"
}

// Provider returns the LLM provider type.
func (m *DeterministicChatModelStub) Provider() string {
	return "deterministic"
}

func (m *DeterministicChatModelStub) toolCall(index int, toolName string, input map[string]any, reason string) ToolCall {
	if toolName == "safe_shell" {
		return ToolCall{}
	}
	call := ToolCall{
		ID:       fmt.Sprintf("call-%03d", index),
		ToolName: toolName,
		Input:    input,
		Reason:   reason,
	}
	call.SchemaCall = schema.ToolCall{
		ID:   call.ID,
		Type: "function",
		Function: schema.FunctionCall{
			Name:      toolName,
			Arguments: mustJSON(input),
		},
	}
	return call
}

func callsFromPlan(plan agent.Plan) []ToolCall {
	calls := make([]ToolCall, 0, len(plan.Steps))
	for index, step := range plan.Steps {
		if step.ToolName == "safe_shell" {
			continue
		}
		callID := fmt.Sprintf("call-%03d", index+1)
		calls = append(calls, ToolCall{
			ID:       callID,
			ToolName: step.ToolName,
			Input:    step.Input,
			Reason:   step.Reason,
			SchemaCall: schema.ToolCall{
				ID:   callID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      step.ToolName,
					Arguments: mustJSON(step.Input),
				},
			},
		})
	}
	return calls
}

func planFromToolCalls(task string, scenario string, summary string, calls []ToolCall, registry *tools.Registry) agent.Plan {
	steps := make([]agent.PlanStep, 0, len(calls))
	for _, call := range calls {
		if call.ToolName == "" {
			continue
		}
		step := agent.PlanStep{
			StepID:   fmt.Sprintf("plan-%03d", len(steps)+1),
			ToolName: call.ToolName,
			Input:    call.Input,
			Reason:   call.Reason,
		}
		if registry != nil {
			if metadata, ok := registry.GetTool(call.ToolName); ok {
				step.ToolCategory = metadata.Category
				step.RiskLevel = metadata.RiskLevel
				step.PermissionScope = metadata.PermissionScope
			}
		}
		steps = append(steps, step)
	}
	return agent.Plan{
		Task:     task,
		Scenario: scenario,
		Summary:  summary,
		Steps:    steps,
	}
}

func ToolMetadataToEinoTool(metadata tools.ToolMetadata) *schema.ToolInfo {
	return &schema.ToolInfo{
		Name:        metadata.Name,
		Desc:        metadata.Description,
		Extra:       map[string]any{"category": metadata.Category, "risk_level": metadata.RiskLevel},
		ParamsOneOf: paramsFromInputSchema(metadata.InputSchema),
	}
}

func paramsFromInputSchema(inputSchema map[string]any) *schema.ParamsOneOf {
	properties, _ := inputSchema["properties"].(map[string]any)
	required := map[string]bool{}
	for _, item := range anySlice(inputSchema["required"]) {
		if name, ok := item.(string); ok {
			required[name] = true
		}
	}
	params := map[string]*schema.ParameterInfo{}
	for name, raw := range properties {
		property, _ := raw.(map[string]any)
		params[name] = parameterFromSchema(property, required[name])
	}
	if len(params) == 0 {
		return nil
	}
	return schema.NewParamsOneOfByParams(params)
}

func parameterFromSchema(property map[string]any, required bool) *schema.ParameterInfo {
	param := &schema.ParameterInfo{Type: schema.String, Required: required}
	if description, ok := property["description"].(string); ok {
		param.Desc = description
	}
	if typeName, ok := property["type"].(string); ok {
		switch typeName {
		case "object":
			param.Type = schema.Object
		case "array":
			param.Type = schema.Array
			param.ElemInfo = &schema.ParameterInfo{Type: schema.String}
		case "integer":
			param.Type = schema.Integer
		case "number":
			param.Type = schema.Number
		case "boolean":
			param.Type = schema.Boolean
		default:
			param.Type = schema.String
		}
	}
	return param
}

func anySlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []string:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}

func normalizeTask(task string) string {
	return strings.Join(strings.Fields(strings.ToLower(task)), " ")
}

func isDeterministicSSHAnomalyTask(task string) bool {
	return strings.Contains(task, "ssh 登录异常") ||
		strings.Contains(task, "登录失败") ||
		strings.Contains(task, "异常登录") ||
		strings.Contains(task, "ssh login anomaly") ||
		strings.Contains(task, "ssh login anomalies") ||
		strings.Contains(task, "ssh auth log") ||
		strings.Contains(task, "ssh authentication") ||
		strings.Contains(task, "failed ssh login") ||
		strings.Contains(task, "ssh failed login") ||
		strings.Contains(task, "suspicious ssh") ||
		strings.Contains(task, "ssh login security") ||
		strings.Contains(task, "inspect ssh") ||
		strings.Contains(task, "analyze ssh") ||
		strings.Contains(task, "analyze failed ssh")
}

func isDeterministicSSHDServiceTask(task string) bool {
	return (strings.Contains(task, "sshd") || strings.Contains(task, "ssh")) &&
		(strings.Contains(task, "服务状态") || strings.Contains(task, "service status") || strings.Contains(task, "check service"))
}

func isDeterministicPort22Task(task string) bool {
	return strings.Contains(task, "22") &&
		(strings.Contains(task, "端口") || strings.Contains(task, "port"))
}

func isDeterministicSystemSecurityOverviewTask(task string) bool {
	return strings.Contains(task, "执行一次系统安全巡检") ||
		strings.Contains(task, "做一次系统安全巡检") ||
		strings.Contains(task, "系统安全检查") ||
		strings.Contains(task, "系统状态巡检") ||
		strings.Contains(task, "system security overview") ||
		strings.Contains(task, "system security check")
}

func isDeterministicSystemResourceCheckTask(task string) bool {
	return strings.Contains(task, "检查当前系统资源使用情况") ||
		strings.Contains(task, "查看系统负载") ||
		strings.Contains(task, "查看 cpu 内存状态") ||
		strings.Contains(task, "检查内存和负载") ||
		strings.Contains(task, "system resource check")
}

func isDeterministicNetworkConnectionCheckTask(task string) bool {
	return strings.Contains(task, "检查当前系统网络连接") ||
		strings.Contains(task, "查看监听端口") ||
		strings.Contains(task, "查看网络连接状态") ||
		strings.Contains(task, "检查异常端口暴露") ||
		strings.Contains(task, "network connection check")
}

func isDeterministicProcessHealthCheckTask(task string) bool {
	return (strings.Contains(task, "检查") || strings.Contains(task, "查看")) &&
		strings.Contains(task, "sshd") &&
		(strings.Contains(task, "进程状态") || strings.Contains(task, "process") ||
			strings.Contains(task, "进程"))
}

func isDeterministicJournalLogCheckTask(task string) bool {
	return (strings.Contains(task, "查看") || strings.Contains(task, "读取")) &&
		strings.Contains(task, "sshd") &&
		(strings.Contains(task, "最近日志") || strings.Contains(task, "服务日志") ||
			strings.Contains(task, "journal") || strings.Contains(task, "systemd 日志"))
}

func mustJSON(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(body)
}
