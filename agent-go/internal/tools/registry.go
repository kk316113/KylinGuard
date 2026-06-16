package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/logtrace"
)

type Handler func(ctx context.Context, input map[string]any) (output any, outputSummary string, riskHint string, err error)

type Registry struct {
	tools    map[string]Handler
	metadata map[string]ToolMetadata
}

type Result struct {
	Output any
	Trace  logtrace.ToolTrace
}

func NewRegistry() *Registry {
	return &Registry{
		tools:    make(map[string]Handler),
		metadata: make(map[string]ToolMetadata),
	}
}

func NewDefaultRegistry() *Registry {
	registry := NewRegistry()
	metadata := DefaultToolMetadata()
	registry.RegisterWithMetadata("os_info", OSInfo, metadata["os_info"])
	registry.RegisterWithMetadata("service_status", ServiceStatus, metadata["service_status"])
	registry.RegisterWithMetadata("log_reader", LogReader, metadata["log_reader"])
	registry.RegisterWithMetadata("ssh_login_analyzer", SSHLoginAnalyzer, metadata["ssh_login_analyzer"])
	registry.RegisterWithMetadata("port_checker", PortChecker, metadata["port_checker"])
	registry.RegisterWithMetadata("safe_shell", SafeShell, metadata["safe_shell"])
	registry.RegisterWithMetadata("process_inspector", ProcessInspector, metadata["process_inspector"])
	registry.RegisterWithMetadata("network_connection_inspector", NetworkConnectionInspector, metadata["network_connection_inspector"])
	registry.RegisterWithMetadata("journalctl_reader", JournalctlReader, metadata["journalctl_reader"])
	registry.RegisterWithMetadata("resource_usage_checker", ResourceUsageChecker, metadata["resource_usage_checker"])
	registry.RegisterWithMetadata("disk_memory_checker", DiskMemoryChecker, metadata["disk_memory_checker"])
	return registry
}

func (r *Registry) Register(name string, handler Handler) {
	r.tools[name] = handler
}

func (r *Registry) RegisterWithMetadata(name string, handler Handler, metadata ToolMetadata) {
	r.Register(name, handler)
	if metadata.Name == "" {
		metadata.Name = name
	}
	r.metadata[name] = metadata
}

func (r *Registry) ListTools() []ToolMetadata {
	if r == nil {
		return []ToolMetadata{}
	}
	names := make([]string, 0, len(r.metadata))
	for name := range r.metadata {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]ToolMetadata, 0, len(names))
	for _, name := range names {
		result = append(result, r.metadata[name])
	}
	return result
}

func (r *Registry) GetTool(name string) (ToolMetadata, bool) {
	if r == nil {
		return ToolMetadata{}, false
	}
	metadata, ok := r.metadata[name]
	return metadata, ok
}

func (r *Registry) IsToolEnabledForDirectCall(name string) bool {
	metadata, ok := r.GetTool(name)
	if !ok {
		return false
	}
	return metadata.Enabled && metadata.DirectCallAllowed && metadata.AllowedByPolicy && !metadata.Dangerous
}

func (r *Registry) CallTool(ctx context.Context, name string, input map[string]any) (Result, error) {
	return r.Invoke(ctx, name, input)
}

func (r *Registry) Invoke(ctx context.Context, name string, input map[string]any) (Result, error) {
	return r.InvokeWithStepID(ctx, "", name, input)
}

func (r *Registry) InvokeWithStepID(ctx context.Context, stepID string, name string, input map[string]any) (Result, error) {
	if input == nil {
		input = map[string]any{}
	}
	if stepID == "" {
		stepID = logtrace.NextStepID()
	}

	startedAt := time.Now().UTC()
	trace := logtrace.ToolTrace{
		StepID:    stepID,
		ToolName:  name,
		Input:     input,
		Status:    "ok",
		StartedAt: startedAt,
		RiskHint:  "low",
	}
	semantic := SemanticForTool(name, input)
	applySemantic(&trace, semantic)

	handler, ok := r.tools[name]
	if !ok {
		err := fmt.Errorf("tool %q is not registered", name)
		trace.Status = "error"
		trace.OutputSummary = err.Error()
		trace.FinishedAt = time.Now().UTC()
		return Result{Trace: trace}, err
	}

	output, summary, riskHint, err := handler(ctx, input)
	applyOutputSemantic(&trace, output)
	trace.OutputSummary = summary
	if riskHint != "" {
		trace.RiskHint = riskHint
	}
	if err != nil {
		trace.Status = "error"
		if trace.OutputSummary == "" {
			trace.OutputSummary = err.Error()
		}
	}
	trace.FinishedAt = time.Now().UTC()

	return Result{Output: output, Trace: trace}, err
}

func applyOutputSemantic(trace *logtrace.ToolTrace, output any) {
	switch typed := output.(type) {
	case SSHLoginAnalyzerResult:
		trace.ResourcePath = sshAuthResourcePath(typed.LogCollection)
	case *SSHLoginAnalyzerResult:
		if typed != nil {
			trace.ResourcePath = sshAuthResourcePath(typed.LogCollection)
		}
	}
	// Extract execution_context from tool results that carry one.
	if ec := extractExecutionContext(output); ec != nil {
		clone := *ec
		trace.ExecutionContext = &clone
	}
}

// extractExecutionContext pulls an ExecutionContext from tool result structs.
func extractExecutionContext(output any) *logtrace.ExecutionContext {
	switch typed := output.(type) {
	case ProcessInspectorResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *ProcessInspectorResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case NetworkConnectionInspectorResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *NetworkConnectionInspectorResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case JournalctlReaderResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *JournalctlReaderResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case ResourceUsageCheckerResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *ResourceUsageCheckerResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case DiskMemoryCheckerResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *DiskMemoryCheckerResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case ServiceStatusResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *ServiceStatusResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case OSInfoResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *OSInfoResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case LogReaderResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *LogReaderResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case PortCheckerResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *PortCheckerResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	}
	return nil
}

func applySemantic(trace *logtrace.ToolTrace, semantic ToolSemantic) {
	trace.OperationType = semantic.OperationType
	trace.ResourceType = semantic.ResourceType
	trace.ResourcePath = semantic.ResourcePath
	trace.PermissionScope = semantic.PermissionScope
	trace.BoundaryLevel = semantic.BoundaryLevel
	trace.ToolSemantic = semantic.ToolSemantic
	trace.RequiresPrivilege = semantic.RequiresPrivilege
	trace.AllowedByPolicy = semantic.AllowedByPolicy
	trace.PolicyReason = semantic.PolicyReason
}

func stringValue(input map[string]any, key string, fallback string) string {
	value, ok := input[key]
	if !ok || value == nil {
		return fallback
	}

	switch typed := value.(type) {
	case string:
		if trimmed := strings.TrimSpace(typed); trimmed != "" {
			return trimmed
		}
	case fmt.Stringer:
		if trimmed := strings.TrimSpace(typed.String()); trimmed != "" {
			return trimmed
		}
	}

	return fallback
}

func intValue(input map[string]any, key string, fallback int) int {
	value, ok := input[key]
	if !ok || value == nil {
		return fallback
	}

	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}

	return fallback
}
