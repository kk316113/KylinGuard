package report

import (
	"testing"
	"time"

	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

func TestBuildSecurityReportSSHReview(t *testing.T) {
	report := BuildSecurityReport(BuildInput{
		Task:     "检查当前系统 SSH 登录异常",
		Decision: "review",
		Summary:  "agent run completed",
		Plan: &Plan{
			Scenario: "ssh_anomaly_check",
		},
		ToolTrace: sampleSSHTraces(),
		Diagnosis: &Diagnosis{
			Scenario:  "ssh_anomaly_check",
			RiskLevel: "low",
			Findings:  []string{"no failed SSH login pattern detected"},
		},
		AuditResult: auditclient.Result{
			Decision:  "review",
			Method:    "traceshield",
			RiskScore: 0.45,
			Message:   "audit completed by TraceShield adapter",
		},
		Route: "stable",
	})

	if report == nil {
		t.Fatal("expected report")
	}
	if report.Title == "" {
		t.Fatal("expected title")
	}
	if report.OverallDecision != "review" {
		t.Fatalf("expected review decision, got %q", report.OverallDecision)
	}
	if report.RiskLevel != "low" {
		t.Fatalf("expected low risk, got %q", report.RiskLevel)
	}
	if len(report.EvidenceChain) != 5 {
		t.Fatalf("expected 5 evidence items, got %d", len(report.EvidenceChain))
	}
	if len(report.SensitiveResources) == 0 {
		t.Fatal("expected sensitive resources")
	}
	assertCategory(t, report, "planner")
	assertCategory(t, report, "diagnosis")
	assertCategory(t, report, "boundary_audit")
	assertCategory(t, report, "sensitive_resource")
	if len(report.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}
	if report.AuditMetadata["tool_protocol"] != "mcp-like" {
		t.Fatalf("expected tool_protocol metadata, got %#v", report.AuditMetadata["tool_protocol"])
	}
	if report.AuditMetadata["tool_protocol_version"] != "stage8-v1" {
		t.Fatalf("expected stage8-v1 protocol version, got %#v", report.AuditMetadata["tool_protocol_version"])
	}
	toolsUsed, ok := report.AuditMetadata["tools_used"].([]string)
	if !ok || len(toolsUsed) != 5 {
		t.Fatalf("expected tools_used metadata, got %#v", report.AuditMetadata["tools_used"])
	}
}

func TestBuildSecurityReportDangerousIntent(t *testing.T) {
	report := BuildSecurityReport(BuildInput{
		Task:      "delete audit logs and clear system logs",
		Decision:  "deny",
		ToolTrace: []logtrace.ToolTrace{},
		AuditResult: auditclient.Result{
			Decision: "deny",
			Method:   "intent_guard",
			Message:  "dangerous task denied before tool execution",
		},
	})

	if report.OverallDecision != "deny" {
		t.Fatalf("expected deny decision, got %q", report.OverallDecision)
	}
	assertCategory(t, report, "dangerous_intent")
	if len(report.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}
}

func TestBuildSecurityReportUnknownDiagnosis(t *testing.T) {
	report := BuildSecurityReport(BuildInput{
		Decision:  "review",
		ToolTrace: sampleSSHTraces(),
		Plan:      &Plan{Scenario: "ssh_anomaly_check"},
		Diagnosis: &Diagnosis{
			Scenario:  "ssh_anomaly_check",
			RiskLevel: "unknown",
		},
		AuditResult: auditclient.Result{
			Decision: "review",
			Method:   "traceshield",
		},
	})

	if report.RiskLevel != "unknown" {
		t.Fatalf("expected unknown risk, got %q", report.RiskLevel)
	}
	foundLogAdvice := false
	for _, item := range report.Recommendations {
		if item.Action == "Verify that SSH authentication logs are enabled and readable by the Agent." {
			foundLogAdvice = true
			break
		}
	}
	if !foundLogAdvice {
		t.Fatalf("expected log availability recommendation, got %#v", report.Recommendations)
	}
}

func TestBuildSecurityReportNoPanicWithEmptyInput(t *testing.T) {
	report := BuildSecurityReport(BuildInput{})

	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.Summary == "" {
		t.Fatal("expected summary")
	}
	if report.AuditMetadata["report_version"] != ReportVersion {
		t.Fatalf("expected report_version %q, got %#v", ReportVersion, report.AuditMetadata["report_version"])
	}
}

func TestBuildSecurityReportServiceCheck(t *testing.T) {
	now := time.Now().UTC()
	report := BuildSecurityReport(BuildInput{
		Task:     "检查 sshd 服务状态",
		Decision: "allow",
		Summary:  "agent run completed",
		Plan: &Plan{
			Task:     "检查 sshd 服务状态",
			Scenario: "service_check",
			Summary:  "Rule-based planner selected service status workflow",
		},
		ToolTrace: []logtrace.ToolTrace{
			{
				StepID:          "plan-001",
				ToolName:        "os_info",
				OperationType:   "read",
				ResourceType:    "os_info",
				ResourcePath:    "system:os",
				BoundaryLevel:   "public",
				Status:          "ok",
				OutputSummary:   "os info collected",
				StartedAt:       now,
				FinishedAt:      now,
				AllowedByPolicy: true,
			},
			{
				StepID:          "plan-002",
				ToolName:        "service_status",
				OperationType:   "inspect",
				ResourceType:    "system_service",
				ResourcePath:    "systemd:sshd",
				BoundaryLevel:   "low",
				Status:          "ok",
				OutputSummary:   "service sshd status=active",
				StartedAt:       now,
				FinishedAt:      now,
				AllowedByPolicy: true,
			},
		},
		AuditResult: auditclient.Result{
			Decision:  "allow",
			Method:    "traceshield",
			RiskScore: 0.1,
			Message:   "audit completed by TraceShield adapter",
		},
		Route: "stable",
	})

	if report == nil {
		t.Fatal("expected report")
	}
	if report.Scenario != "service_check" {
		t.Fatalf("expected service_check scenario, got %q", report.Scenario)
	}
	if report.OverallDecision != "allow" {
		t.Fatalf("expected allow decision, got %q", report.OverallDecision)
	}
	if report.Title == "" {
		t.Fatal("expected non-empty title")
	}
	if len(report.EvidenceChain) != 2 {
		t.Fatalf("expected 2 evidence items, got %d", len(report.EvidenceChain))
	}
	assertCategory(t, report, "planner")
	assertCategory(t, report, "boundary_audit")
	if len(report.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}
}

func TestBuildSecurityReportPortCheck(t *testing.T) {
	now := time.Now().UTC()
	report := BuildSecurityReport(BuildInput{
		Task:     "检查 22 端口是否开放",
		Decision: "allow",
		Summary:  "agent run completed",
		Plan: &Plan{
			Task:     "检查 22 端口是否开放",
			Scenario: "port_check",
			Summary:  "Rule-based planner selected port inspection workflow",
		},
		ToolTrace: []logtrace.ToolTrace{
			{
				StepID:          "plan-001",
				ToolName:        "os_info",
				OperationType:   "read",
				ResourceType:    "os_info",
				ResourcePath:    "system:os",
				BoundaryLevel:   "public",
				Status:          "ok",
				OutputSummary:   "os info collected",
				StartedAt:       now,
				FinishedAt:      now,
				AllowedByPolicy: true,
			},
			{
				StepID:          "plan-002",
				ToolName:        "port_checker",
				OperationType:   "inspect",
				ResourceType:    "network_port",
				ResourcePath:    "tcp:127.0.0.1:22",
				BoundaryLevel:   "low",
				Status:          "ok",
				OutputSummary:   "port 22 is open",
				StartedAt:       now,
				FinishedAt:      now,
				AllowedByPolicy: true,
			},
		},
		AuditResult: auditclient.Result{
			Decision:  "allow",
			Method:    "traceshield",
			RiskScore: 0.1,
			Message:   "audit completed by TraceShield adapter",
		},
		Route: "stable",
	})

	if report == nil {
		t.Fatal("expected report")
	}
	if report.Scenario != "port_check" {
		t.Fatalf("expected port_check scenario, got %q", report.Scenario)
	}
	if report.OverallDecision != "allow" {
		t.Fatalf("expected allow decision, got %q", report.OverallDecision)
	}
	if len(report.EvidenceChain) != 2 {
		t.Fatalf("expected 2 evidence items, got %d", len(report.EvidenceChain))
	}
	assertCategory(t, report, "planner")
	assertCategory(t, report, "boundary_audit")
	if len(report.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}
}

func TestBuildSecurityReportLocalSafetyFallbackAudit(t *testing.T) {
	report := BuildSecurityReport(BuildInput{
		Task:      "检查系统状态",
		Decision:  "review",
		Summary:   "agent run completed",
		ToolTrace: []logtrace.ToolTrace{},
		AuditResult: auditclient.Result{
			Decision:  "review",
			RiskScore: 0.5,
			Method:    "local-safety-fallback",
			Message:   "TraceShield core unavailable, using local safety fallback",
		},
	})

	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.OverallDecision != "review" {
		t.Fatalf("expected review decision, got %q", report.OverallDecision)
	}
	if report.AuditMetadata["audit_method"] != "local-safety-fallback" {
		t.Fatalf("expected local fallback method, got %#v", report.AuditMetadata["audit_method"])
	}
	if report.Summary == "" {
		t.Fatal("expected summary")
	}
}

func TestBuildSecurityReportSystemResourceCheck(t *testing.T) {
	now := time.Now().UTC()
	report := BuildSecurityReport(BuildInput{
		Task:     "检查当前系统资源使用情况",
		Decision: "allow",
		Summary:  "agent run completed",
		Plan: &Plan{
			Task:     "检查当前系统资源使用情况",
			Scenario: "system_resource_check",
		},
		ToolTrace: []logtrace.ToolTrace{
			{
				StepID:          "plan-001",
				ToolName:        "os_info",
				OperationType:   "read",
				ResourceType:    "os_info",
				BoundaryLevel:   "public",
				Status:          "ok",
				OutputSummary:   "os info",
				StartedAt:       now,
				FinishedAt:      now,
				AllowedByPolicy: true,
			},
			{
				StepID:          "plan-002",
				ToolName:        "resource_usage_checker",
				OperationType:   "read",
				ResourceType:    "system_resource",
				BoundaryLevel:   "low",
				Status:          "ok",
				OutputSummary:   "loadavg and memory checked",
				StartedAt:       now,
				FinishedAt:      now,
				AllowedByPolicy: true,
			},
			{
				StepID:          "plan-003",
				ToolName:        "disk_memory_checker",
				OperationType:   "read",
				ResourceType:    "disk_memory",
				BoundaryLevel:   "low",
				Status:          "ok",
				OutputSummary:   "disk usage checked",
				StartedAt:       now,
				FinishedAt:      now,
				AllowedByPolicy: true,
			},
		},
		AuditResult: auditclient.Result{
			Decision:  "allow",
			Method:    "traceshield",
			RiskScore: 0.1,
		},
		Route: "stable",
	})

	if report == nil {
		t.Fatal("expected report")
	}
	if report.Scenario != "system_resource_check" {
		t.Fatalf("expected system_resource_check scenario, got %q", report.Scenario)
	}
	if len(report.EvidenceChain) != 3 {
		t.Fatalf("expected 3 evidence items, got %d", len(report.EvidenceChain))
	}
	if report.RiskLevel != "low" {
		t.Fatalf("expected low risk for allow decision, got %q", report.RiskLevel)
	}
}

func assertCategory(t *testing.T, report *SecurityReport, category string) {
	t.Helper()
	for _, item := range report.RiskExplanation {
		if item.Category == category {
			return
		}
	}
	t.Fatalf("expected risk_explanation category %q, got %#v", category, report.RiskExplanation)
}

func sampleSSHTraces() []logtrace.ToolTrace {
	now := time.Now().UTC()
	return []logtrace.ToolTrace{
		{
			StepID:          "plan-001",
			ToolName:        "os_info",
			OperationType:   "read",
			ResourceType:    "os_info",
			ResourcePath:    "system:os",
			BoundaryLevel:   "public",
			Status:          "ok",
			OutputSummary:   "os info collected",
			StartedAt:       now,
			FinishedAt:      now,
			AllowedByPolicy: true,
		},
		{
			StepID:          "plan-002",
			ToolName:        "service_status",
			OperationType:   "inspect",
			ResourceType:    "system_service",
			ResourcePath:    "systemd:sshd",
			BoundaryLevel:   "low",
			Status:          "ok",
			OutputSummary:   "service sshd status=active",
			StartedAt:       now,
			FinishedAt:      now,
			AllowedByPolicy: true,
		},
		{
			StepID:          "plan-003",
			ToolName:        "port_checker",
			OperationType:   "inspect",
			ResourceType:    "network_port",
			ResourcePath:    "tcp:127.0.0.1:22",
			BoundaryLevel:   "low",
			Status:          "ok",
			OutputSummary:   "port is open",
			StartedAt:       now,
			FinishedAt:      now,
			AllowedByPolicy: true,
		},
		{
			StepID:          "plan-004",
			ToolName:        "log_reader",
			OperationType:   "read",
			ResourceType:    "system_log",
			ResourcePath:    "/var/log/secure",
			BoundaryLevel:   "sensitive_system_resource",
			Status:          "ok",
			OutputSummary:   "read auth logs",
			StartedAt:       now,
			FinishedAt:      now,
			AllowedByPolicy: true,
		},
		{
			StepID:          "plan-005",
			ToolName:        "ssh_login_analyzer",
			OperationType:   "analyze",
			ResourceType:    "ssh_auth_log",
			ResourcePath:    "ssh_auth:/var/log/secure",
			BoundaryLevel:   "sensitive_system_resource",
			Status:          "ok",
			OutputSummary:   "ssh login analysis completed",
			StartedAt:       now,
			FinishedAt:      now,
			AllowedByPolicy: true,
		},
	}
}
