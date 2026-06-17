package reasoningtrace

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var spanSequence uint64

// SpanType categorises each span in the reasoning trace.
type SpanType string

const (
	SpanRequest            SpanType = "request"
	SpanIntentGuard        SpanType = "intent_guard"
	SpanPlanner            SpanType = "planner"
	SpanChatModel          SpanType = "chat_model"
	SpanToolPolicy         SpanType = "tool_policy"
	SpanExecProxy          SpanType = "exec_proxy"
	SpanToolCall           SpanType = "tool_call"
	SpanAudit              SpanType = "audit"
	SpanDecisionNormalizer SpanType = "decision_normalizer"
	SpanDiagnosis          SpanType = "diagnosis"
	SpanSecurityReport     SpanType = "security_report"
)

// ReasoningEvent is a timestamped event within a span.
type ReasoningEvent struct {
	Name       string         `json:"name"`
	Timestamp  time.Time      `json:"timestamp"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// ReasoningSpan is a single phase of the reasoning pipeline.
type ReasoningSpan struct {
	SpanID       string           `json:"span_id"`
	ParentSpanID string           `json:"parent_span_id,omitempty"`
	Type         SpanType         `json:"type"`
	Name         string           `json:"name"`
	Status       string           `json:"status"`
	StartedAt    time.Time        `json:"started_at"`
	EndedAt      time.Time        `json:"ended_at"`
	DurationMs   int64            `json:"duration_ms"`
	Attributes   map[string]any   `json:"attributes,omitempty"`
	Events       []ReasoningEvent `json:"events,omitempty"`
}

// ReasoningTrace is the top-level object returned alongside agent responses.
type ReasoningTrace struct {
	TraceID     string          `json:"trace_id"`
	Runtime     string          `json:"runtime"`
	TaskHash    string          `json:"task_hash"`
	TaskSummary string          `json:"task_summary"`
	StartedAt   time.Time       `json:"started_at"`
	EndedAt     time.Time       `json:"ended_at"`
	DurationMs  int64           `json:"duration_ms"`
	Spans       []ReasoningSpan `json:"spans"`
}

// NextSpanID generates a unique span identifier.
func NextSpanID() string {
	next := atomic.AddUint64(&spanSequence, 1)
	return fmt.Sprintf("span-%06d", next)
}

// NewReasoningTrace creates a new reasoning trace.
func NewReasoningTrace(runtime string, task string) *ReasoningTrace {
	now := time.Now().UTC()
	return &ReasoningTrace{
		TraceID:     fmt.Sprintf("trace-%d", now.UnixNano()),
		Runtime:     runtime,
		TaskHash:    taskHash(task),
		TaskSummary: truncate(task, 120),
		StartedAt:   now,
		Spans:       []ReasoningSpan{},
	}
}

// Finish sets the end time and duration of the trace.
func (rt *ReasoningTrace) Finish() {
	rt.EndedAt = time.Now().UTC()
	rt.DurationMs = msSince(rt.StartedAt, rt.EndedAt)
}

// AddSpan appends a completed span to the trace.
func (rt *ReasoningTrace) AddSpan(span ReasoningSpan) {
	rt.Spans = append(rt.Spans, span)
}

// taskHash produces a simple hash of the task string for dedup without storing the full task.
func taskHash(task string) string {
	h := 0
	for _, c := range task {
		h = (h*31 + int(c)) & 0x7FFFFFFF
	}
	return fmt.Sprintf("%08x", h)
}

// msSince returns the number of milliseconds between two time points.
func msSince(start, end time.Time) int64 {
	return end.Sub(start).Milliseconds()
}

// truncate truncates a string to maxLen characters, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// NewSpan creates a span with the given type, name, and parent.
func NewSpan(spanType SpanType, name string, parentID string) ReasoningSpan {
	now := time.Now().UTC()
	return ReasoningSpan{
		SpanID:       NextSpanID(),
		ParentSpanID: parentID,
		Type:         spanType,
		Name:         name,
		Status:       "ok",
		StartedAt:    now,
		Attributes:   map[string]any{},
		Events:       []ReasoningEvent{},
	}
}

// FinishSpan updates the span end time and duration.
func FinishSpan(span *ReasoningSpan, status string) {
	if span == nil {
		return
	}
	span.EndedAt = time.Now().UTC()
	span.DurationMs = msSince(span.StartedAt, span.EndedAt)
	if status != "" {
		span.Status = status
	}
}

// AddEventToSpan adds a timestamped event to an existing span.
func AddEventToSpan(span *ReasoningSpan, name string, attrs map[string]any) {
	if span == nil {
		return
	}
	span.Events = append(span.Events, ReasoningEvent{
		Name:       name,
		Timestamp:  time.Now().UTC(),
		Attributes: SanitizeAttributes(attrs),
	})
}

// SetSpanAttribute sets a key-value pair on the span, with redaction.
func SetSpanAttribute(span *ReasoningSpan, key string, value any) {
	if span == nil {
		return
	}
	if span.Attributes == nil {
		span.Attributes = map[string]any{}
	}
	span.Attributes[key] = SanitizeValue(key, value)
}

// TraceBuilder provides a convenient builder for constructing reasoning traces.
type TraceBuilder struct {
	mu    sync.Mutex
	Trace *ReasoningTrace
	spans map[string]*ReasoningSpan
}

// NewTraceBuilder creates a new TraceBuilder.
func NewTraceBuilder(runtime string, task string) *TraceBuilder {
	return &TraceBuilder{
		Trace: NewReasoningTrace(runtime, task),
		spans: make(map[string]*ReasoningSpan),
	}
}

// StartSpan starts a new span and records it in the builder.
func (tb *TraceBuilder) StartSpan(parentID string, spanType SpanType, name string) *ReasoningSpan {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	span := NewSpan(spanType, name, parentID)
	tb.Trace.AddSpan(span)
	tb.spans[span.SpanID] = &tb.Trace.Spans[len(tb.Trace.Spans)-1]
	return &tb.Trace.Spans[len(tb.Trace.Spans)-1]
}

// EndSpan finishes a span with the given status.
func (tb *TraceBuilder) EndSpan(spanID string, status string) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if span, ok := tb.spans[spanID]; ok {
		FinishSpan(span, status)
	}
}

// AddEvent adds an event to an existing span.
func (tb *TraceBuilder) AddEvent(spanID string, name string, attrs map[string]any) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if span, ok := tb.spans[spanID]; ok {
		AddEventToSpan(span, name, SanitizeAttributes(attrs))
	}
}

// SetAttr sets an attribute on an existing span.
func (tb *TraceBuilder) SetAttr(spanID string, key string, value any) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if span, ok := tb.spans[spanID]; ok {
		SetSpanAttribute(span, key, value)
	}
}

// Finish completes the trace and returns it.
func (tb *TraceBuilder) Finish() *ReasoningTrace {
	tb.Trace.Finish()
	return tb.Trace
}

// SpanIDs returns a map of span type -> span ID for the current builder.
func (tb *TraceBuilder) SpanIDs(spanType SpanType) []string {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	var ids []string
	for _, s := range tb.Trace.Spans {
		if s.Type == spanType {
			ids = append(ids, s.SpanID)
		}
	}
	return ids
}

// AddSpanFromBuilder adds a pre-built span (e.g. from a tool call) to the trace.
func (tb *TraceBuilder) AddSpanFromBuilder(span ReasoningSpan) string {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.Trace.AddSpan(span)
	return span.SpanID
}

// CompletionStatus returns a human-readable status string from the reasoning trace.
func (rt *ReasoningTrace) CompletionStatus() string {
	if rt == nil || len(rt.Spans) == 0 {
		return "no_spans"
	}
	// Look for the decision_normalizer span to get the final decision.
	for _, s := range rt.Spans {
		if s.Type == SpanDecisionNormalizer {
			if s.Status == "ok" {
				if v, ok := s.Attributes["normalized_decision"]; ok {
					return fmt.Sprintf("%v", v)
				}
			}
		}
	}
	return "completed"
}

// Summary returns a one-line summary for logging/display.
func (rt *ReasoningTrace) Summary() string {
	if rt == nil {
		return "no reasoning trace"
	}
	return fmt.Sprintf("trace=%s runtime=%s spans=%d duration=%dms task=%s",
		rt.TraceID, rt.Runtime, len(rt.Spans), rt.DurationMs, rt.TaskSummary)
}
