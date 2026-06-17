package reasoningtrace

import (
	"testing"
	"time"
)

func TestNewReasoningTrace(t *testing.T) {
	rt := NewReasoningTrace("stable", "test task description")
	if rt == nil {
		t.Fatal("expected non-nil ReasoningTrace")
	}
	if rt.TraceID == "" {
		t.Fatal("expected trace_id to be non-empty")
	}
	if rt.Runtime != "stable" {
		t.Fatalf("expected runtime=stable, got %q", rt.Runtime)
	}
	if rt.TaskSummary != "test task description" {
		t.Fatalf("expected task_summary='test task description', got %q", rt.TaskSummary)
	}
	if rt.TaskHash == "" {
		t.Fatal("expected task_hash to be non-empty")
	}
	if len(rt.Spans) != 0 {
		t.Fatalf("expected no spans initially, got %d", len(rt.Spans))
	}
}

func TestNewReasoningTraceTruncatesLongTask(t *testing.T) {
	longTask := ""
	for i := 0; i < 200; i++ {
		longTask += "x"
	}
	rt := NewReasoningTrace("eino", longTask)
	if len(rt.TaskSummary) > 130 {
		t.Fatalf("task_summary too long: %d chars", len(rt.TaskSummary))
	}
}

func TestTraceBuilderLifecycle(t *testing.T) {
	tb := NewTraceBuilder("test-runtime", "test task")
	span1 := tb.StartSpan("", SpanRequest, "request span")
	if span1.SpanID == "" {
		t.Fatal("expected span1 to have span_id")
	}
	if span1.ParentSpanID != "" {
		t.Fatalf("expected empty parent_span_id for root span, got %q", span1.ParentSpanID)
	}
	if span1.Type != SpanRequest {
		t.Fatalf("expected type=request, got %q", span1.Type)
	}
	if span1.Status != "ok" {
		t.Fatalf("expected initial status=ok, got %q", span1.Status)
	}

	// Start a child span.
	span2 := tb.StartSpan(span1.SpanID, SpanIntentGuard, "intent_guard")
	if span2.ParentSpanID != span1.SpanID {
		t.Fatalf("expected parent_span_id=%s, got %q", span1.SpanID, span2.ParentSpanID)
	}

	// Add attributes.
	tb.SetAttr(span2.SpanID, "decision", "allow")
	tb.SetAttr(span2.SpanID, "blocked_reason", "none")

	// End spans.
	tb.EndSpan(span2.SpanID, "allow")
	tb.EndSpan(span1.SpanID, "completed")

	trace := tb.Finish()
	if trace == nil {
		t.Fatal("expected non-nil trace")
	}
	if trace.DurationMs < 0 {
		t.Fatal("expected duration_ms >= 0")
	}
	if len(trace.Spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(trace.Spans))
	}
	// Verify span durations are set (can be 0 for fast-spawning).
	for _, s := range trace.Spans {
		if s.DurationMs < 0 {
			t.Fatalf("span %s duration should be >=0, got %d", s.SpanID, s.DurationMs)
		}
	}
}

func TestSpanDurationAfterEndSpan(t *testing.T) {
	tb := NewTraceBuilder("test", "duration test")
	sp := tb.StartSpan("", SpanRequest, "test")
	time.Sleep(time.Millisecond)
	tb.EndSpan(sp.SpanID, "ok")
	if sp.DurationMs < 1 {
		t.Fatalf("expected duration_ms >= 1 after sleep, got %d", sp.DurationMs)
	}
}

func TestSanitizeSensitiveKeys(t *testing.T) {
	tb := NewTraceBuilder("test", "sanitize test")
	sp := tb.StartSpan("", SpanRequest, "test")

	sensitive := map[string]string{
		"api_key":     "sk-1234567890abcdef",
		"authorization": "Bearer eyJhbGciOiJIUzI1NiIs...",
		"token":       "my-secret-token",
		"password":    "supersecret",
		"secret":      "my-secret-value",
		"credential":  "user:pass",
		"private_key": "-----BEGIN RSA PRIVATE KEY-----\nMII",
	}
	for k, v := range sensitive {
		tb.SetAttr(sp.SpanID, k, v)
	}
	tb.EndSpan(sp.SpanID, "ok")
	tb.Finish()

	for k := range sensitive {
		v := sp.Attributes[k]
		if v != "[REDACTED]" {
			t.Fatalf("expected attribute %q to be [REDACTED], got %v", k, v)
		}
	}
}

func TestSanitizeBearerValues(t *testing.T) {
	tb := NewTraceBuilder("test", "bearer test")
	sp := tb.StartSpan("", SpanRequest, "test")

	tb.SetAttr(sp.SpanID, "raw_token", "Bearer sk-abcdef123456")
	tb.EndSpan(sp.SpanID, "ok")
	tb.Finish()

	v := sp.Attributes["raw_token"]
	if v != "[REDACTED]" {
		t.Fatalf("expected attribute to be [REDACTED] for Bearer value, got %v", v)
	}
}

func TestAddEvent(t *testing.T) {
	tb := NewTraceBuilder("test", "event test")
	sp := tb.StartSpan("", SpanRequest, "test")
	tb.AddEvent(sp.SpanID, "policy_check", map[string]any{"tool": "port_checker", "allowed": true})
	tb.EndSpan(sp.SpanID, "ok")
	tb.Finish()

	if len(sp.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(sp.Events))
	}
	ev := sp.Events[0]
	if ev.Name != "policy_check" {
		t.Fatalf("expected event name=policy_check, got %q", ev.Name)
	}
	if ev.Attributes["allowed"] != true {
		t.Fatalf("expected event attribute allowed=true, got %v", ev.Attributes["allowed"])
	}
}

func TestTruncatedSummary(t *testing.T) {
	short := "hello world"
	if s := TruncatedSummary(short, 100); s != short {
		t.Fatalf("expected %q, got %q", short, s)
	}

	long := ""
	for i := 0; i < 500; i++ {
		long += "x"
	}
	s := TruncatedSummary(long, 200)
	if len(s) > 220 {
		t.Fatalf("truncated summary too long: %d chars", len(s))
	}
}

func TestSanitizeAttributes(t *testing.T) {
	attrs := map[string]any{
		"tool":      "port_checker",
		"api_key":   "sk-abc",
		"authorization": "Bearer token123",
		"safe_key":  "safe_value",
	}
	sanitized := SanitizeAttributes(attrs)
	if sanitized["tool"] != "port_checker" {
		t.Fatalf("expected safe key to remain unchanged")
	}
	if sanitized["api_key"] != "[REDACTED]" {
		t.Fatalf("expected api_key to be REDACTED")
	}
	if sanitized["authorization"] != "[REDACTED]" {
		t.Fatalf("expected authorization to be REDACTED")
	}
}

func TestCompletionStatus(t *testing.T) {
	tb := NewTraceBuilder("test", "status test")
	sp := tb.StartSpan("", SpanRequest, "request")
	tb.EndSpan(sp.SpanID, "ok")
	tb.Finish()

	status := tb.Trace.CompletionStatus()
	if status == "no_spans" {
		t.Fatal("expected completion status to be set")
	}
}

func TestTraceFinishSetsDuration(t *testing.T) {
	rt := NewReasoningTrace("test", "duration")
	time.Sleep(time.Millisecond)
	rt.Finish()
	if rt.DurationMs < 1 {
		t.Fatalf("expected duration_ms >= 1, got %d", rt.DurationMs)
	}
	if rt.EndedAt.Before(rt.StartedAt) {
		t.Fatal("ended_at should be after started_at")
	}
}
