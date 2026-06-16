package report

import (
	"fmt"
	"strings"

	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/tools"
)

type anyToolTrace struct {
	ToolName      string
	OperationType string
	ResourceType  string
	ResourcePath  string
	BoundaryLevel string
}

func BuildSecurityReport(input BuildInput) *SecurityReport {
	report := &SecurityReport{
		Title:              titleFor(input),
		Scenario:           scenarioFromInput(input),
		OverallDecision:    fallback(input.Decision, fallback(input.AuditResult.Decision, "review")),
		RiskLevel:          riskLevelFromInput(input),
		EvidenceChain:      buildEvidenceChain(input.ToolTrace),
		SensitiveResources: buildSensitiveResources(input.ToolTrace),
		Recommendations:    []RecommendationItem{},
		RiskExplanation:    []RiskExplanationItem{},
		AuditMetadata:      map[string]any{},
	}

	report.RiskExplanation = buildRiskExplanation(input, report)
	report.Recommendations = buildRecommendations(input, report)
	report.Summary = summaryFor(input, report)
	report.AuditMetadata = buildAuditMetadata(input, report)
	if report.Summary == "" {
		report.Summary = "KylinGuard generated a deterministic audit report for the current request."
	}
	if report.Title == "" {
		report.Title = "KylinGuard Security Audit Report"
	}
	if report.RiskLevel == "" {
		report.RiskLevel = "unknown"
	}
	return report
}

func buildEvidenceChain(traces []logtrace.ToolTrace) []EvidenceItem {
	evidence := make([]EvidenceItem, 0, len(traces))
	for index, trace := range traces {
		view := anyToolTrace{
			ToolName:      trace.ToolName,
			OperationType: trace.OperationType,
			ResourceType:  trace.ResourceType,
			ResourcePath:  trace.ResourcePath,
			BoundaryLevel: trace.BoundaryLevel,
		}
		evidence = append(evidence, EvidenceItem{
			EvidenceID:    fmt.Sprintf("E-%03d", index+1),
			StepID:        trace.StepID,
			ToolName:      trace.ToolName,
			OperationType: trace.OperationType,
			ResourceType:  trace.ResourceType,
			ResourcePath:  trace.ResourcePath,
			BoundaryLevel: trace.BoundaryLevel,
			Status:        trace.Status,
			Summary:       trace.OutputSummary,
			WhyRelevant:   whyRelevant(view),
			AuditMeaning:  auditMeaning(view),
		})
	}
	return evidence
}

func buildSensitiveResources(traces []logtrace.ToolTrace) []SensitiveResourceItem {
	result := []SensitiveResourceItem{}
	seen := map[string]bool{}
	for _, trace := range traces {
		if !isSensitiveTrace(trace) {
			continue
		}
		key := trace.ResourceType + "|" + trace.ResourcePath + "|" + trace.BoundaryLevel
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, SensitiveResourceItem{
			ResourceType:    trace.ResourceType,
			ResourcePath:    trace.ResourcePath,
			BoundaryLevel:   trace.BoundaryLevel,
			AccessReason:    accessReason(trace.ToolName, trace.ResourceType),
			AllowedByPolicy: trace.AllowedByPolicy,
		})
	}
	return result
}

func isSensitiveTrace(trace logtrace.ToolTrace) bool {
	switch trace.BoundaryLevel {
	case "sensitive_system_resource", "dangerous", "privileged", "high":
		return true
	}
	resource := strings.ToLower(trace.ResourceType)
	for _, token := range []string{"system_log", "ssh_auth_log", "audit_log", "secret", "credential"} {
		if strings.Contains(resource, token) {
			return true
		}
	}
	return false
}

func buildRiskExplanation(input BuildInput, report *SecurityReport) []RiskExplanationItem {
	items := []RiskExplanationItem{}
	if report.Scenario != "" {
		items = appendRisk(items, "info", "planner", "Planner selected scenario "+report.Scenario+" and generated a multi-step security diagnosis workflow.", nil)
	}
	if isDangerousIntent(input) {
		items = appendRisk(items, "high", "dangerous_intent", "Intent Guard denied the request before tool execution because it matched dangerous operational intent.", nil)
	}
	if len(report.SensitiveResources) > 0 {
		items = appendRisk(items, "medium", "sensitive_resource", "The task accessed sensitive system resources such as system_log or ssh_auth_log, so the operation requires audit review.", sensitiveEvidenceIDs(report.EvidenceChain))
	}
	if input.Diagnosis != nil {
		items = appendRisk(items, diagnosisSeverity(input.Diagnosis.RiskLevel), "diagnosis", diagnosisDescription(input.Diagnosis.RiskLevel), evidenceIDsForTool(report.EvidenceChain, "ssh_login_analyzer"))
	}
	if input.AuditResult.Method == "traceshield" {
		severity := "info"
		if report.OverallDecision == "deny" {
			severity = "high"
		} else if report.OverallDecision == "review" {
			severity = "info"
		}
		items = appendRisk(items, severity, "boundary_audit", "TraceShield audited the semantic tool-call chain and produced the final audit result.", allEvidenceIDs(report.EvidenceChain))
	}
	if len(items) == 0 {
		items = appendRisk(items, "info", "boundary_audit", "KylinGuard generated a minimal report even though audit context was incomplete.", nil)
	}
	return items
}

func appendRisk(items []RiskExplanationItem, severity string, category string, description string, evidenceIDs []string) []RiskExplanationItem {
	return append(items, RiskExplanationItem{
		ReasonID:    fmt.Sprintf("RISK-%03d", len(items)+1),
		Severity:    severity,
		Category:    category,
		Description: description,
		EvidenceIDs: evidenceIDs,
	})
}

func buildRecommendations(input BuildInput, report *SecurityReport) []RecommendationItem {
	recs := []RecommendationItem{}
	if isDangerousIntent(input) {
		recs = appendRecommendation(recs, "high", "Do not execute log deletion or audit cleanup requests through the Agent.", "The request matched destructive operational intent and was blocked before tool execution.")
		recs = appendRecommendation(recs, "medium", "Review the original user request and require explicit administrative approval for destructive maintenance actions.", "Destructive maintenance needs explicit human approval outside the normal diagnosis workflow.")
	}

	if input.Diagnosis != nil {
		switch input.Diagnosis.RiskLevel {
		case "low":
			recs = appendRecommendation(recs, "low", "Continue monitoring SSH authentication logs.", "The current diagnosis did not find an obvious SSH brute-force pattern.")
			recs = appendRecommendation(recs, "low", "Keep sshd logging and audit collection enabled.", "Continued log collection keeps future diagnosis explainable.")
		case "medium":
			recs = appendRecommendation(recs, "medium", "Review repeated failed SSH source IPs and verify whether they are expected.", "Repeated failed login attempts can indicate scanning or password guessing.")
			recs = appendRecommendation(recs, "medium", "Consider restricting SSH exposure to trusted networks or enabling rate-limit protection.", "Exposure reduction and rate limiting can reduce SSH attack surface after human review.")
		case "high":
			recs = appendRecommendation(recs, "high", "Immediately review high-frequency failed SSH source IPs.", "High-frequency failures may indicate active brute-force behavior.")
			recs = appendRecommendation(recs, "high", "Inspect successful login events around the suspicious time window.", "Successful logins near repeated failures need human review.")
			recs = appendRecommendation(recs, "medium", "Consider temporary SSH access restriction after human approval.", "Access restrictions should be reviewed and approved before execution.")
		default:
			recs = appendRecommendation(recs, "medium", "Verify that SSH authentication logs are enabled and readable by the Agent.", "The diagnosis risk level is unknown because authentication logs were unavailable or insufficient.")
			recs = appendRecommendation(recs, "low", "Check /var/log/secure, /var/log/auth.log, and journalctl -u sshd availability.", "These are the current SSH log sources used by KylinGuard.")
		}
	}

	if len(report.SensitiveResources) > 0 {
		recs = appendRecommendation(recs, "medium", "Keep sensitive log access scoped to user-requested security diagnosis tasks.", "Sensitive system resources should remain tied to explicit diagnostic intent.")
	}
	if len(recs) == 0 {
		recs = appendRecommendation(recs, "low", "Review the generated tool trace and audit result before taking operational action.", "The report is explanatory and does not execute remediation.")
	}
	return recs
}

func appendRecommendation(items []RecommendationItem, priority string, action string, rationale string) []RecommendationItem {
	return append(items, RecommendationItem{
		RecommendationID: fmt.Sprintf("REC-%03d", len(items)+1),
		Priority:         priority,
		Action:           action,
		Rationale:        rationale,
		IsDestructive:    false,
	})
}

func buildAuditMetadata(input BuildInput, report *SecurityReport) map[string]any {
	route := input.Route
	if route == "" {
		route = "stable"
	}
	return map[string]any{
		"audit_method":             input.AuditResult.Method,
		"audit_message":            input.AuditResult.Message,
		"audit_risk_score":         input.AuditResult.RiskScore,
		"trace_count":              len(input.ToolTrace),
		"evidence_count":           len(report.EvidenceChain),
		"sensitive_resource_count": len(report.SensitiveResources),
		"has_diagnosis":            input.Diagnosis != nil,
		"route":                    route,
		"generated_by":             "kylin-guard-report-builder",
		"report_version":           ReportVersion,
		"tool_protocol":            tools.ToolProtocol,
		"tool_protocol_version":    tools.ToolProtocolVersion,
		"registered_tool_count":    tools.RegisteredToolCount(),
		"tools_used":               toolsUsed(input.ToolTrace),
	}
}

func toolsUsed(traces []logtrace.ToolTrace) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, trace := range traces {
		if trace.ToolName == "" || seen[trace.ToolName] {
			continue
		}
		seen[trace.ToolName] = true
		result = append(result, trace.ToolName)
	}
	return result
}

func riskLevelFromInput(input BuildInput) string {
	if isDangerousIntent(input) {
		return "high"
	}
	if input.Diagnosis != nil && input.Diagnosis.RiskLevel != "" {
		return input.Diagnosis.RiskLevel
	}
	switch input.AuditResult.Decision {
	case "deny":
		return "high"
	case "review":
		return "medium"
	case "allow":
		return "low"
	}
	return "unknown"
}

func scenarioFromInput(input BuildInput) string {
	if input.Plan != nil && input.Plan.Scenario != "" {
		return input.Plan.Scenario
	}
	if input.Diagnosis != nil && input.Diagnosis.Scenario != "" {
		return input.Diagnosis.Scenario
	}
	return ""
}

func isDangerousIntent(input BuildInput) bool {
	return input.AuditResult.Method == "intent_guard" || (input.Decision == "deny" && len(input.ToolTrace) == 0)
}

func sensitiveEvidenceIDs(evidence []EvidenceItem) []string {
	ids := []string{}
	for _, item := range evidence {
		if item.BoundaryLevel == "sensitive_system_resource" || strings.Contains(strings.ToLower(item.ResourceType), "log") {
			ids = append(ids, item.EvidenceID)
		}
	}
	return ids
}

func evidenceIDsForTool(evidence []EvidenceItem, toolName string) []string {
	ids := []string{}
	for _, item := range evidence {
		if item.ToolName == toolName {
			ids = append(ids, item.EvidenceID)
		}
	}
	return ids
}

func allEvidenceIDs(evidence []EvidenceItem) []string {
	ids := make([]string, 0, len(evidence))
	for _, item := range evidence {
		ids = append(ids, item.EvidenceID)
	}
	return ids
}

func fallback(value string, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
