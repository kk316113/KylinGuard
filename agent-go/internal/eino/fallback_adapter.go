package eino

import (
	"context"
	"fmt"
	"sync"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/tools"
)

type fallbackOutcomeKey struct{}

// FallbackOutcome is request-scoped so concurrent runs cannot overwrite each
// other's model metadata.
type FallbackOutcome struct {
	mu     sync.RWMutex
	used   bool
	reason string
}

func withFallbackOutcome(ctx context.Context) (context.Context, *FallbackOutcome) {
	outcome := &FallbackOutcome{}
	return context.WithValue(ctx, fallbackOutcomeKey{}, outcome), outcome
}

func fallbackOutcomeFromContext(ctx context.Context) *FallbackOutcome {
	outcome, _ := ctx.Value(fallbackOutcomeKey{}).(*FallbackOutcome)
	return outcome
}

func (o *FallbackOutcome) record(used bool, reason string) {
	if o == nil {
		return
	}
	if !used {
		return
	}
	o.mu.Lock()
	o.used, o.reason = true, reason
	o.mu.Unlock()
}

func (o *FallbackOutcome) Info() (bool, string) {
	if o == nil {
		return false, ""
	}
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.used, o.reason
}

// FallbackChatModelAdapter is stateless; fallback metadata is stored in the
// request context rather than on this shared adapter.
type FallbackChatModelAdapter struct {
	primary  ChatModelAdapter
	fallback ChatModelAdapter
}

func NewFallbackChatModelAdapter(primary ChatModelAdapter, registry *tools.Registry) *FallbackChatModelAdapter {
	return &FallbackChatModelAdapter{primary: primary, fallback: NewDeterministicChatModelStub(registry)}
}

func (f *FallbackChatModelAdapter) GenerateToolCalls(ctx context.Context, task string, toolDefs []tools.ToolMetadata) ([]ToolCall, agent.Plan, error) {
	calls, plan, err := f.primary.GenerateToolCalls(ctx, task, toolDefs)
	if err == nil {
		fallbackOutcomeFromContext(ctx).record(false, "")
		return calls, plan, nil
	}
	reason := fmt.Sprintf("primary adapter failed: %v; falling back to deterministic stub", err)
	fallbackOutcomeFromContext(ctx).record(true, reason)
	return f.fallback.GenerateToolCalls(ctx, task, toolDefs)
}

func (f *FallbackChatModelAdapter) Name() string     { return f.primary.Name() }
func (f *FallbackChatModelAdapter) Provider() string { return f.primary.Provider() }
