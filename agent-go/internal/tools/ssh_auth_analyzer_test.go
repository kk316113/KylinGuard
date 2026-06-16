package tools

import "testing"

func TestAnalyzeSSHAuthLinesHighRiskTopIP(t *testing.T) {
	lines := []string{
		"Jun 16 10:00:01 host sshd[1]: Accepted publickey for root from 192.168.1.10 port 2222 ssh2",
		"Jun 16 10:00:02 host sshd[1]: Failed password for invalid user admin from 10.0.0.8 port 33333 ssh2",
	}
	for i := 0; i < 10; i++ {
		lines = append(lines, "Jun 16 10:00:03 host sshd[1]: Failed password for root from 192.168.1.5 port 54321 ssh2")
	}

	analysis := AnalyzeSSHAuthLines(lines)

	if analysis.FailedTotal != 11 {
		t.Fatalf("expected 11 failed attempts, got %d", analysis.FailedTotal)
	}
	if analysis.AcceptedTotal != 1 {
		t.Fatalf("expected 1 accepted login, got %d", analysis.AcceptedTotal)
	}
	if analysis.InvalidUserTotal != 1 {
		t.Fatalf("expected 1 invalid user attempt, got %d", analysis.InvalidUserTotal)
	}
	if analysis.RiskLevel != "high" {
		t.Fatalf("expected high risk, got %q", analysis.RiskLevel)
	}
	if len(analysis.TopFailedIPs) == 0 || analysis.TopFailedIPs[0].IP != "192.168.1.5" || analysis.TopFailedIPs[0].FailedCount != 10 {
		t.Fatalf("unexpected top failed IPs: %#v", analysis.TopFailedIPs)
	}
}

func TestAnalyzeSSHAuthLinesMediumRiskInvalidUsers(t *testing.T) {
	lines := []string{
		"Invalid user test1 from 10.0.0.1 port 1001",
		"Invalid user test2 from 10.0.0.2 port 1002",
		"Invalid user test3 from 10.0.0.3 port 1003",
		"Invalid user test4 from 10.0.0.4 port 1004",
		"Invalid user test5 from 10.0.0.5 port 1005",
	}

	analysis := AnalyzeSSHAuthLines(lines)

	if analysis.InvalidUserTotal != 5 {
		t.Fatalf("expected 5 invalid user attempts, got %d", analysis.InvalidUserTotal)
	}
	if analysis.RiskLevel != "medium" {
		t.Fatalf("expected medium risk, got %q", analysis.RiskLevel)
	}
}

func TestAnalyzeSSHAuthLinesLowRiskNoFailures(t *testing.T) {
	analysis := AnalyzeSSHAuthLines([]string{
		"Accepted password for user from 192.168.1.20 port 2020 ssh2",
	})

	if analysis.FailedTotal != 0 {
		t.Fatalf("expected no failures, got %d", analysis.FailedTotal)
	}
	if analysis.AcceptedTotal != 1 {
		t.Fatalf("expected accepted login, got %d", analysis.AcceptedTotal)
	}
	if analysis.RiskLevel != "low" {
		t.Fatalf("expected low risk, got %q", analysis.RiskLevel)
	}
}

func TestAnalyzeSSHAuthLinesUnknownWithoutLogs(t *testing.T) {
	analysis := AnalyzeSSHAuthLines(nil)

	if analysis.RiskLevel != "unknown" {
		t.Fatalf("expected unknown risk, got %q", analysis.RiskLevel)
	}
	if len(analysis.Findings) == 0 {
		t.Fatal("expected finding for unavailable logs")
	}
}

func TestAnalyzeSSHAuthLinesFailureWithoutIP(t *testing.T) {
	analysis := AnalyzeSSHAuthLines([]string{
		"authentication failure; logname= uid=0 euid=0 tty=ssh ruser= rhost=",
	})

	if analysis.FailedTotal != 1 {
		t.Fatalf("expected one failure, got %d", analysis.FailedTotal)
	}
	if len(analysis.TopFailedIPs) != 0 {
		t.Fatalf("expected no top IPs when no IP can be extracted, got %#v", analysis.TopFailedIPs)
	}
	if analysis.RiskLevel != "low" {
		t.Fatalf("expected low risk for single unattributed failure, got %q", analysis.RiskLevel)
	}
}
