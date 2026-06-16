package tools

import (
	"net"
	"regexp"
	"sort"
	"strings"
)

type IPFailureStat struct {
	IP          string `json:"ip"`
	FailedCount int    `json:"failed_count"`
}

type SSHAuthAnalysis struct {
	FailedTotal      int             `json:"failed_total"`
	AcceptedTotal    int             `json:"accepted_total"`
	InvalidUserTotal int             `json:"invalid_user_total"`
	TopFailedIPs     []IPFailureStat `json:"top_failed_ips"`
	RiskLevel        string          `json:"risk_level"`
	Findings         []string        `json:"findings"`
}

var sshFromIPPattern = regexp.MustCompile(`\bfrom\s+([0-9A-Fa-f:.]+)\b`)

func AnalyzeSSHAuthLines(lines []string) SSHAuthAnalysis {
	analysis := SSHAuthAnalysis{
		TopFailedIPs: []IPFailureStat{},
		RiskLevel:    "unknown",
		Findings:     []string{},
	}
	if len(lines) == 0 {
		analysis.Findings = append(analysis.Findings, "no auth log lines available")
		return analysis
	}

	failuresByIP := map[string]int{}
	for _, line := range lines {
		lower := strings.ToLower(line)
		failed := false

		if strings.Contains(lower, "failed password") {
			analysis.FailedTotal++
			failed = true
		}
		if strings.Contains(lower, "authentication failure") {
			analysis.FailedTotal++
			failed = true
		}
		if strings.Contains(lower, "failed password for invalid user") || strings.Contains(lower, "invalid user") {
			analysis.InvalidUserTotal++
		}
		if strings.Contains(lower, "accepted password") || strings.Contains(lower, "accepted publickey") {
			analysis.AcceptedTotal++
		}
		if failed {
			if ip := extractSSHLogIP(line); ip != "" {
				failuresByIP[ip]++
			}
		}
	}

	analysis.TopFailedIPs = topFailedIPs(failuresByIP, 5)
	analysis.RiskLevel = classifySSHRisk(analysis)
	analysis.Findings = sshFindings(analysis)
	return analysis
}

func extractSSHLogIP(line string) string {
	matches := sshFromIPPattern.FindStringSubmatch(line)
	if len(matches) < 2 {
		return ""
	}
	candidate := strings.Trim(matches[1], "[]")
	if parsed := net.ParseIP(candidate); parsed != nil {
		return candidate
	}
	return ""
}

func topFailedIPs(failuresByIP map[string]int, limit int) []IPFailureStat {
	stats := make([]IPFailureStat, 0, len(failuresByIP))
	for ip, count := range failuresByIP {
		stats = append(stats, IPFailureStat{IP: ip, FailedCount: count})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].FailedCount == stats[j].FailedCount {
			return stats[i].IP < stats[j].IP
		}
		return stats[i].FailedCount > stats[j].FailedCount
	})
	if len(stats) > limit {
		stats = stats[:limit]
	}
	return stats
}

func classifySSHRisk(analysis SSHAuthAnalysis) string {
	maxFailures := 0
	if len(analysis.TopFailedIPs) > 0 {
		maxFailures = analysis.TopFailedIPs[0].FailedCount
	}
	switch {
	case maxFailures >= 10:
		return "high"
	case maxFailures >= 5:
		return "medium"
	case analysis.InvalidUserTotal >= 5:
		return "medium"
	case analysis.FailedTotal == 0:
		return "low"
	default:
		return "low"
	}
}

func sshFindings(analysis SSHAuthAnalysis) []string {
	findings := []string{}
	if analysis.FailedTotal == 0 {
		findings = append(findings, "no failed SSH login pattern detected")
	} else {
		findings = append(findings, "failed SSH login attempts detected")
	}
	if analysis.InvalidUserTotal > 0 {
		findings = append(findings, "invalid SSH user attempts detected")
	}
	if len(analysis.TopFailedIPs) > 0 {
		findings = append(findings, "top failed SSH source IPs aggregated")
	}
	if analysis.AcceptedTotal > 0 {
		findings = append(findings, "successful SSH login events observed")
	}
	return findings
}
