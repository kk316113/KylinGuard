package agent

import (
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/security"
)

// NormalizeAgentDecision normalizes the audit decision returned by TraceShield for read-only OS sensing tools.
// TraceShield may return "deny" for tool names it does not recognize even when the tool
// is a safe read/inspect operation. This normalizer preserves genuine deny decisions
// (intent_guard, tool_policy, dangerous operations) while mapping overly conservative
// TraceShield denials to "allow" or "review" based on the actual tool trace semantics.
func NormalizeAgentDecision(auditDecision string, auditMethod string, traces []logtrace.ToolTrace, diagnosisRiskLevel string) string {
	// Rule A: Intent guard deny — always preserve.
	if auditMethod == "intent_guard" {
		return string(security.DecisionDeny)
	}

	// Rule B: Tool policy deny — always preserve.
	if auditMethod == "tool_policy" {
		return string(security.DecisionDeny)
	}

	// If the audit method is not traceshield (e.g., fallback-mock), trust it as-is.
	if auditMethod != "traceshield" {
		return auditDecision
	}

	// If there are no traces (should not happen in a non-denied path), trust the audit decision.
	if len(traces) == 0 {
		return auditDecision
	}

	// Check for dangerous operations in the trace — if any tool is marked dangerous,
	// has a denied policy, or performs execute operations, preserve deny.
	if containsDangerousOperation(traces) {
		return string(security.DecisionDeny)
	}

	// If any tool was denied by policy, preserve deny.
	if anyPolicyDenied(traces) {
		return string(security.DecisionDeny)
	}

	// If the audit result is "allow" or "review", trust it — no normalization needed.
	if auditDecision == string(security.DecisionAllow) || auditDecision == string(security.DecisionReview) {
		return auditDecision
	}

	// At this point, auditDecision is "deny" but the tools are all read-only, allowed by policy,
	// and not dangerous. This is the case we need to fix: TraceShield returned "deny"
	// for unrecognized read-only tools. Normalize based on sensitivity.

	// All tools are read/inspect/analyze with allowed_by_policy=true.
	// Check sensitivity level.
	if containsSensitiveBoundary(traces) {
		return string(security.DecisionReview)
	}

	// High diagnosis risk should still be review.
	if diagnosisRiskLevel == "high" {
		return string(security.DecisionReview)
	}

	return string(security.DecisionAllow)
}

// containsDangerousOperation checks if any trace step contains a dangerous operation.
func containsDangerousOperation(traces []logtrace.ToolTrace) bool {
	for _, trace := range traces {
		// Explicit dangerous flag.
		if trace.OperationType == "execute" {
			// safe_shell is explicitly denied at policy level, so any execute
			// that reached traceshield should be treated as dangerous.
			return true
		}
		// If any step was not allowed by policy, it's dangerous.
		if !trace.AllowedByPolicy {
			return true
		}
		// If boundary is dangerous or privileged.
		if trace.BoundaryLevel == "dangerous" || trace.BoundaryLevel == "privileged" {
			return true
		}
	}
	return false
}

// anyPolicyDenied checks if any trace step was denied by policy.
func anyPolicyDenied(traces []logtrace.ToolTrace) bool {
	for _, trace := range traces {
		if !trace.AllowedByPolicy {
			return true
		}
	}
	return false
}

// containsSensitiveBoundary checks if any trace step touches sensitive system resources.
func containsSensitiveBoundary(traces []logtrace.ToolTrace) bool {
	for _, trace := range traces {
		if trace.BoundaryLevel == "sensitive_system_resource" {
			return true
		}
		if trace.RequiresPrivilege {
			return true
		}
	}
	return false
}
