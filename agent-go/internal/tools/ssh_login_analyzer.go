package tools

import (
	"context"
	"fmt"
)

type SSHLoginAnalyzerResult struct {
	LogCollection LogCollectionResult `json:"log_collection"`
	Analysis      SSHAuthAnalysis     `json:"analysis"`
}

func SSHLoginAnalyzer(ctx context.Context, input map[string]any) (any, string, string, error) {
	collection, collectErr := collectSSHAuthLogs(ctx, input)
	analysis := AnalyzeSSHAuthLines(collection.Lines)
	result := SSHLoginAnalyzerResult{
		LogCollection: collection,
		Analysis:      analysis,
	}

	summary := fmt.Sprintf(
		"ssh login analysis completed: risk=%s failed=%d accepted=%d invalid_users=%d source=%s",
		analysis.RiskLevel,
		analysis.FailedTotal,
		analysis.AcceptedTotal,
		analysis.InvalidUserTotal,
		sshAuthResourcePath(collection),
	)
	if collectErr != nil {
		summary = "ssh login analysis completed with unavailable logs: " + collection.Message
	}

	return result, summary, riskHintForSSHRisk(analysis.RiskLevel), collectErr
}

func riskHintForSSHRisk(riskLevel string) string {
	switch riskLevel {
	case "high":
		return "high"
	case "medium":
		return "review"
	case "low":
		return "low"
	default:
		return "review"
	}
}

func sshAuthResourcePath(collection LogCollectionResult) string {
	switch collection.SourceType {
	case "file":
		if collection.SourcePath != "" {
			return "ssh_auth:" + collection.SourcePath
		}
	case "journalctl":
		return "ssh_auth:journalctl:sshd"
	}
	return "ssh_auth:none"
}
