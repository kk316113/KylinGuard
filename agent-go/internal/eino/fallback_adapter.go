package eino

import (
	"context"
	"fmt"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/tools"
)

// FallbackChatModelAdapter wraps a primary adapter and falls back to a deterministic stub on failure.
// It records the fallback reason so the runtime can inject it into the reasoning trace.
type FallbackChatModelAdapter struct {
	primary        ChatModelAdapter
	fallback       ChatModelAdapter
	usedFallback   bool
	fallbackReason string
}

func NewFallbackChatModelAdapter(primary ChatModelAdapter, registry *tools.Registry) *FallbackChatModelAdapter {
	return &FallbackChatModelAdapter{
		primary:  primary,
		fallback: NewDeterministicChatModelStub(registry),
	}
}

func (f *FallbackChatModelAdapter) GenerateToolCalls(ctx context.Context, task string, toolDefs []tools.ToolMetadata) ([]ToolCall, agent.Plan, error) {
	f.usedFallback = false
	f.fallbackReason = ""

	// Try the primary adapter first.
	calls, plan, err := f.primary.GenerateToolCalls(ctx, task, toolDefs)
	if err == nil {
		return calls, plan, nil
	}

	// Primary failed — fall back to deterministic stub.
	f.usedFallback = true
	f.fallbackReason = fmt.Sprintf("primary adapter failed: %v; falling back to deterministic stub", err)

	return f.fallback.GenerateToolCalls(ctx, task, toolDefs)
}

func (f *FallbackChatModelAdapter) Name() string {
	if f.usedFallback {
		return f.fallback.Name()
	}
	return f.primary.Name()
}

func (f *FallbackChatModelAdapter) Provider() string {
	if f.usedFallback {
		return f.fallback.Provider()
	}
	return f.primary.Provider()
}

// FallbackInfo returns whether the fallback was used and why.
func (f *FallbackChatModelAdapter) FallbackInfo() (bool, string) {
	return f.usedFallback, f.fallbackReason
}
