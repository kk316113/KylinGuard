package agent

import (
	"context"
	"errors"
)

var (
	ErrEinoAdapterDisabled       = errors.New("eino adapter disabled")
	ErrEinoRuntimeNotImplemented = errors.New("eino real runtime not implemented")
)

type EinoAdapter struct {
	requestedEnabled bool
}

func NewEinoAdapter(requestedEnabled bool) EinoAdapter {
	return EinoAdapter{requestedEnabled: requestedEnabled}
}

func (a EinoAdapter) Run(ctx context.Context, req AgentRunRequest) (AgentRunResponse, error) {
	_ = ctx
	_ = req
	if a.requestedEnabled {
		return AgentRunResponse{}, ErrEinoRuntimeNotImplemented
	}
	return AgentRunResponse{}, ErrEinoAdapterDisabled
}

func (EinoAdapter) Name() string {
	return "eino_adapter"
}

func (EinoAdapter) Enabled() bool {
	// Stage 3 intentionally avoids importing Eino until the package path and Kylin build story are confirmed.
	return false
}

func (a EinoAdapter) FallbackSummary() string {
	if a.requestedEnabled {
		return "eino real runtime not implemented, stable runtime fallback used"
	}
	return "eino adapter disabled, stable runtime fallback used"
}
