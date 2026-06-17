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

// --- Kylin/OpenSSH common log format tests ---

func TestAnalyzeSSHAuthLinesKylinSecureLogFormat(t *testing.T) {
	// Kylin V11 /var/log/secure typical format
	lines := []string{
		"Jun 16 08:12:33 kylin-host sshd[12345]: Accepted publickey for root from 10.0.0.1 port 49230 ssh2",
		"Jun 16 08:12:45 kylin-host sshd[12346]: Failed password for root from 192.168.1.100 port 51001 ssh2",
		"Jun 16 08:12:50 kylin-host sshd[12347]: Failed password for root from 192.168.1.100 port 51002 ssh2",
		"Jun 16 08:13:01 kylin-host sshd[12348]: Failed password for invalid user test from 192.168.1.100 port 51003 ssh2",
		"Jun 16 08:13:10 kylin-host sshd[12349]: Accepted password for admin from 10.0.0.2 port 60001 ssh2",
	}

	analysis := AnalyzeSSHAuthLines(lines)

	if analysis.FailedTotal != 3 {
		t.Fatalf("expected 3 failures, got %d", analysis.FailedTotal)
	}
	if analysis.AcceptedTotal != 2 {
		t.Fatalf("expected 2 accepted, got %d", analysis.AcceptedTotal)
	}
	if analysis.InvalidUserTotal != 1 {
		t.Fatalf("expected 1 invalid user, got %d", analysis.InvalidUserTotal)
	}
	if analysis.RiskLevel != "low" {
		t.Fatalf("expected low risk (3 failures from one IP, below medium threshold), got %q", analysis.RiskLevel)
	}
	if len(analysis.TopFailedIPs) == 0 || analysis.TopFailedIPs[0].IP != "192.168.1.100" {
		t.Fatalf("expected top failed IP 192.168.1.100, got %#v", analysis.TopFailedIPs)
	}
}

func TestAnalyzeSSHAuthLinesKylinAuthLogFormat(t *testing.T) {
	// /var/log/auth.log format (Debian/Kylin)
	lines := []string{
		"Jun 16 09:00:01 host sshd[100]: pam_unix(sshd:auth): authentication failure; logname= uid=0 euid=0 tty=ssh ruser= rhost=10.0.0.5  user=root",
		"Jun 16 09:00:02 host sshd[100]: Failed password for root from 10.0.0.5 port 12345 ssh2",
		"Jun 16 09:00:03 host sshd[101]: pam_unix(sshd:auth): authentication failure; logname= uid=0 euid=0 tty=ssh ruser= rhost=10.0.0.5",
		"Jun 16 09:00:04 host sshd[101]: Failed password for root from 10.0.0.5 port 12346 ssh2",
	}

	analysis := AnalyzeSSHAuthLines(lines)

	if analysis.FailedTotal != 4 {
		t.Fatalf("expected 4 failures (2 pam + 2 failed password), got %d", analysis.FailedTotal)
	}
	if len(analysis.TopFailedIPs) == 0 || analysis.TopFailedIPs[0].IP != "10.0.0.5" {
		t.Fatalf("expected top failed IP 10.0.0.5, got %#v", analysis.TopFailedIPs)
	}
	if analysis.TopFailedIPs[0].FailedCount < 2 {
		t.Fatalf("expected at least 2 failures from 10.0.0.5, got %d", analysis.TopFailedIPs[0].FailedCount)
	}
}

func TestAnalyzeSSHAuthLinesJournalctlFormat(t *testing.T) {
	// journalctl -u sshd output format
	lines := []string{
		"Jun 16 10:00:00 host sshd[200]: Accepted publickey for ops from 172.16.0.1 port 33000 ssh2",
		"Jun 16 10:00:05 host sshd[201]: Connection closed by 172.16.0.1 port 33000 [preauth]",
		"Jun 16 10:01:00 host sshd[202]: Failed password for root from 203.0.113.50 port 40001 ssh2",
	}

	analysis := AnalyzeSSHAuthLines(lines)

	if analysis.AcceptedTotal != 1 {
		t.Fatalf("expected 1 accepted, got %d", analysis.AcceptedTotal)
	}
	if analysis.FailedTotal != 1 {
		t.Fatalf("expected 1 failure, got %d", analysis.FailedTotal)
	}
	if analysis.RiskLevel != "low" {
		t.Fatalf("expected low risk, got %q", analysis.RiskLevel)
	}
}

func TestAnalyzeSSHAuthLinesMultipleIPBruteForce(t *testing.T) {
	// Simulated brute-force from multiple IPs
	lines := []string{}
	// 5 failures from IP A
	for i := 0; i < 5; i++ {
		lines = append(lines, "Jun 16 11:00:00 host sshd[300]: Failed password for root from 10.10.10.1 port 1000 ssh2")
	}
	// 3 failures from IP B
	for i := 0; i < 3; i++ {
		lines = append(lines, "Jun 16 11:00:01 host sshd[301]: Failed password for admin from 10.10.10.2 port 2000 ssh2")
	}
	// 2 failures from IP C
	for i := 0; i < 2; i++ {
		lines = append(lines, "Jun 16 11:00:02 host sshd[302]: Failed password for user from 10.10.10.3 port 3000 ssh2")
	}

	analysis := AnalyzeSSHAuthLines(lines)

	if analysis.FailedTotal != 10 {
		t.Fatalf("expected 10 failures, got %d", analysis.FailedTotal)
	}
	if analysis.RiskLevel != "medium" {
		t.Fatalf("expected medium risk (5 failures from one IP), got %q", analysis.RiskLevel)
	}
	if len(analysis.TopFailedIPs) != 3 {
		t.Fatalf("expected 3 top IPs, got %d", len(analysis.TopFailedIPs))
	}
	// IP A should be first (highest count)
	if analysis.TopFailedIPs[0].IP != "10.10.10.1" || analysis.TopFailedIPs[0].FailedCount != 5 {
		t.Fatalf("expected top IP 10.10.10.1 with 5 failures, got %#v", analysis.TopFailedIPs[0])
	}
}

func TestAnalyzeSSHAuthLinesIPv6Addresses(t *testing.T) {
	lines := []string{
		"Jun 16 12:00:00 host sshd[400]: Failed password for root from ::1 port 50000 ssh2",
		"Jun 16 12:00:01 host sshd[401]: Failed password for root from 2001:db8::1 port 50001 ssh2",
		"Jun 16 12:00:02 host sshd[402]: Accepted publickey for root from fe80::1 port 50002 ssh2",
	}

	analysis := AnalyzeSSHAuthLines(lines)

	if analysis.FailedTotal != 2 {
		t.Fatalf("expected 2 failures, got %d", analysis.FailedTotal)
	}
	if analysis.AcceptedTotal != 1 {
		t.Fatalf("expected 1 accepted, got %d", analysis.AcceptedTotal)
	}
	if analysis.RiskLevel != "low" {
		t.Fatalf("expected low risk, got %q", analysis.RiskLevel)
	}
}

func TestAnalyzeSSHAuthLinesHighRiskFromSingleIP(t *testing.T) {
	// 15 failures from single IP → high risk
	lines := []string{}
	for i := 0; i < 15; i++ {
		lines = append(lines, "Jun 16 13:00:00 host sshd[500]: Failed password for root from 10.20.30.40 port 60000 ssh2")
	}

	analysis := AnalyzeSSHAuthLines(lines)

	if analysis.FailedTotal != 15 {
		t.Fatalf("expected 15 failures, got %d", analysis.FailedTotal)
	}
	if analysis.RiskLevel != "high" {
		t.Fatalf("expected high risk (15 failures from single IP), got %q", analysis.RiskLevel)
	}
	if len(analysis.TopFailedIPs) != 1 || analysis.TopFailedIPs[0].IP != "10.20.30.40" {
		t.Fatalf("expected single top IP 10.20.30.40, got %#v", analysis.TopFailedIPs)
	}
}

func TestAnalyzeSSHAuthLinesMixedAcceptedAndRejected(t *testing.T) {
	// Mixed pattern: attacker tries, fails, then succeeds (suspicious)
	lines := []string{
		"Jun 16 14:00:00 host sshd[600]: Failed password for root from 10.30.0.1 port 70000 ssh2",
		"Jun 16 14:00:01 host sshd[601]: Failed password for root from 10.30.0.1 port 70001 ssh2",
		"Jun 16 14:00:02 host sshd[602]: Failed password for admin from 10.30.0.1 port 70002 ssh2",
		"Jun 16 14:00:03 host sshd[603]: Accepted password for root from 10.30.0.1 port 70003 ssh2",
	}

	analysis := AnalyzeSSHAuthLines(lines)

	if analysis.FailedTotal != 3 {
		t.Fatalf("expected 3 failures, got %d", analysis.FailedTotal)
	}
	if analysis.AcceptedTotal != 1 {
		t.Fatalf("expected 1 accepted, got %d", analysis.AcceptedTotal)
	}
	if len(analysis.Findings) == 0 {
		t.Fatal("expected findings")
	}
	// Check that both "failed" and "successful" findings exist
	hasFailed := false
	hasAccepted := false
	for _, f := range analysis.Findings {
		if f == "failed SSH login attempts detected" {
			hasFailed = true
		}
		if f == "successful SSH login events observed" {
			hasAccepted = true
		}
	}
	if !hasFailed || !hasAccepted {
		t.Fatalf("expected both failed and accepted findings, got %#v", analysis.Findings)
	}
}

func TestAnalyzeSSHAuthLinesEmptyLines(t *testing.T) {
	analysis := AnalyzeSSHAuthLines([]string{})

	if analysis.RiskLevel != "unknown" {
		t.Fatalf("expected unknown risk for empty lines, got %q", analysis.RiskLevel)
	}
	if len(analysis.Findings) == 0 {
		t.Fatal("expected finding for empty logs")
	}
}

func TestAnalyzeSSHAuthLinesKylinV11RealWorldSample(t *testing.T) {
	// Real-world Kylin V11 /var/log/secure samples
	lines := []string{
		"Jun 15 03:22:01 kylin-server sshd[8612]: Received disconnect from 192.168.1.50 port 39422:11: Bye Bye [preauth]",
		"Jun 15 03:22:01 kylin-server sshd[8612]: Disconnected from authenticating user root 192.168.1.50 port 39422 [preauth]",
		"Jun 15 03:22:01 kylin-server sshd[8610]: pam_unix(sshd:auth): authentication failure; logname= uid=0 euid=0 tty=ssh ruser= rhost=192.168.1.50  user=root",
		"Jun 15 03:22:01 kylin-server sshd[8610]: Failed password for root from 192.168.1.50 port 39420 ssh2",
		"Jun 15 03:22:02 kylin-server sshd[8614]: Connection closed by 192.168.1.50 port 39424 [preauth]",
		"Jun 15 08:15:30 kylin-server sshd[9100]: Accepted publickey for root from 10.0.0.1 port 2222 ssh2",
		"Jun 15 08:15:30 kylin-server sshd[9100]: pam_unix(sshd:session): session opened for user root by (uid=0)",
	}

	analysis := AnalyzeSSHAuthLines(lines)

	if analysis.FailedTotal < 1 {
		t.Fatalf("expected at least 1 failure, got %d", analysis.FailedTotal)
	}
	if analysis.AcceptedTotal != 1 {
		t.Fatalf("expected 1 accepted, got %d", analysis.AcceptedTotal)
	}
	if analysis.RiskLevel != "low" {
		t.Fatalf("expected low risk for typical Kylin log, got %q", analysis.RiskLevel)
	}
}
