package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/logtrace"
)

type Handler func(ctx context.Context, input map[string]any) (output any, outputSummary string, riskHint string, err error)

type Registry struct {
	tools map[string]Handler
}

type Result struct {
	Output any
	Trace  logtrace.ToolTrace
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Handler)}
}

func NewDefaultRegistry() *Registry {
	registry := NewRegistry()
	registry.Register("os_info", OSInfo)
	registry.Register("service_status", ServiceStatus)
	registry.Register("log_reader", LogReader)
	registry.Register("port_checker", PortChecker)
	registry.Register("safe_shell", SafeShell)
	return registry
}

func (r *Registry) Register(name string, handler Handler) {
	r.tools[name] = handler
}

func (r *Registry) Invoke(ctx context.Context, name string, input map[string]any) (Result, error) {
	startedAt := time.Now().UTC()
	trace := logtrace.ToolTrace{
		StepID:    logtrace.NextStepID(),
		ToolName:  name,
		Input:     input,
		Status:    "ok",
		StartedAt: startedAt,
		RiskHint:  "low",
	}

	handler, ok := r.tools[name]
	if !ok {
		err := fmt.Errorf("tool %q is not registered", name)
		trace.Status = "error"
		trace.OutputSummary = err.Error()
		trace.FinishedAt = time.Now().UTC()
		return Result{Trace: trace}, err
	}

	output, summary, riskHint, err := handler(ctx, input)
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
