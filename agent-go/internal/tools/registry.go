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
	registrations := []struct {
		name    string
		handler Handler
	}{
		{"os_info", OSInfo},
		{"service_status", ServiceStatus},
		{"log_reader", LogReader},
		{"ssh_login_analyzer", SSHLoginAnalyzer},
		{"port_checker", PortChecker},
		{"safe_shell", SafeShell},
		{"process_inspector", ProcessInspector},
		{"network_connection_inspector", NetworkConnectionInspector},
		{"journalctl_reader", JournalctlReader},
		{"resource_usage_checker", ResourceUsageChecker},
		{"disk_memory_checker", DiskMemoryChecker},
		{"open_file_inspector", OpenFileInspector},
		{"disk_io_checker", DiskIOChecker},
		{"configuration_drift_detector", ConfigurationDriftDetector},
		{"systemd_unit_inventory", SystemdUnitInventory},
		{"block_device_inventory", BlockDeviceInventory},
		{"mount_inventory", MountInventory},
		{"rpm_package_inventory", RPMPackageInventory},
	}
	for _, entry := range registrations {
		registry.RegisterWithMetadata(entry.name, entry.handler, metadata[entry.name])
	}
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

func (r *Registry) ListDirectCallTools() []ToolMetadata {
	if r == nil {
		return []ToolMetadata{}
	}
	all := r.ListTools()
	result := make([]ToolMetadata, 0, len(all))
	for _, metadata := range all {
		if r.IsToolEnabledForDirectCall(metadata.Name) {
			result = append(result, metadata)
		}
	}
	return result
}

func (r *Registry) RegisteredNames() []string {
	if r == nil {
		return []string{}
	}
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) MetadataNames() []string {
	if r == nil {
		return []string{}
	}
	names := make([]string, 0, len(r.metadata))
	for name := range r.metadata {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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

func (r *Registry) Validate() error {
	if r == nil {
		return fmt.Errorf("tool registry is nil")
	}
	problems := make([]string, 0)
	for _, name := range r.RegisteredNames() {
		if _, ok := r.metadata[name]; !ok {
			problems = append(problems, fmt.Sprintf("registered handler %q has no metadata", name))
		}
	}
	for _, name := range r.MetadataNames() {
		metadata := r.metadata[name]
		if _, ok := r.tools[name]; !ok {
			problems = append(problems, fmt.Sprintf("metadata %q has no handler", name))
		}
		if err := validateToolMetadata(name, metadata); err != nil {
			problems = append(problems, err.Error())
		}
		if r.IsToolEnabledForDirectCall(name) && !metadata.IsReadOnly() {
			problems = append(problems, fmt.Sprintf("direct-call tool %q is not read-only", name))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid tool registry: %s", strings.Join(problems, "; "))
	}
	return nil
}

func validateToolMetadata(name string, metadata ToolMetadata) error {
	switch {
	case strings.TrimSpace(name) == "":
		return fmt.Errorf("tool metadata has empty registry name")
	case strings.TrimSpace(metadata.Name) == "":
		return fmt.Errorf("tool %q missing metadata name", name)
	case metadata.Name != name:
		return fmt.Errorf("tool %q metadata name mismatch: %q", name, metadata.Name)
	case strings.TrimSpace(metadata.Description) == "":
		return fmt.Errorf("tool %q missing description", name)
	case strings.TrimSpace(metadata.Category) == "":
		return fmt.Errorf("tool %q missing category", name)
	case strings.TrimSpace(metadata.Version) == "":
		return fmt.Errorf("tool %q missing version", name)
	case metadata.InputSchema == nil:
		return fmt.Errorf("tool %q missing input_schema", name)
	case metadata.OutputSchema == nil:
		return fmt.Errorf("tool %q missing output_schema", name)
	case strings.TrimSpace(metadata.RiskLevel) == "":
		return fmt.Errorf("tool %q missing risk_level", name)
	case strings.TrimSpace(metadata.PermissionScope) == "":
		return fmt.Errorf("tool %q missing permission_scope", name)
	case strings.TrimSpace(metadata.OperationType) == "":
		return fmt.Errorf("tool %q missing operation_type", name)
	case strings.TrimSpace(metadata.ResourceType) == "":
		return fmt.Errorf("tool %q missing resource_type", name)
	case strings.TrimSpace(metadata.BoundaryLevel) == "":
		return fmt.Errorf("tool %q missing boundary_level", name)
	}
	return nil
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
	case OpenFileInspectorResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *OpenFileInspectorResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case DiskIOCheckerResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *DiskIOCheckerResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case ConfigurationDriftResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *ConfigurationDriftResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case SystemdUnitInventoryResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *SystemdUnitInventoryResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case BlockDeviceInventoryResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *BlockDeviceInventoryResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case MountInventoryResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *MountInventoryResult:
		if typed != nil && typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case RPMPackageInventoryResult:
		if typed.ExecutionContext != nil {
			return typed.ExecutionContext
		}
	case *RPMPackageInventoryResult:
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
