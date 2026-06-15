package tools

import (
	"context"
	"time"
)

type LogReaderResult struct {
	Mode      string    `json:"mode"`
	Message   string    `json:"message"`
	Lines     []string  `json:"lines"`
	Timestamp time.Time `json:"timestamp"`
}

func LogReader(ctx context.Context, input map[string]any) (any, string, string, error) {
	_ = ctx
	_ = input

	result := LogReaderResult{
		Mode:      "stub",
		Message:   "log reader is registered but does not read files in Stage 0",
		Lines:     []string{},
		Timestamp: time.Now().UTC(),
	}
	return result, "log reader stub; no files read", "review", nil
}
