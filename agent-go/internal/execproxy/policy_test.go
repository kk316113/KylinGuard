package execproxy

import (
	"strings"
	"testing"
)

func TestExecPolicyAllowsWhitelistedReadCommand(t *testing.T) {
	policy := NewExecPolicy()

	tests := []struct {
		command string
		args    []string
		profile ExecutionProfile
	}{
		{"ps", []string{"aux"}, ProfileLowRead},
		{"ss", []string{"-tunlp"}, ProfileLowRead},
		{"netstat", []string{"-ano"}, ProfileLowRead},
		{"journalctl", []string{"-u", "sshd", "-n", "100", "--no-pager"}, ProfileSensitiveRead},
		{"df", []string{"-h"}, ProfileLowRead},
		{"uname", []string{"-r"}, ProfilePublicRead},
		{"systemctl", []string{"is-active", "sshd"}, ProfileLowRead},
		{"systemctl", []string{"list-units", "--type=service", "--all", "--no-pager", "--plain", "--legend=false"}, ProfileLowRead},
		{"lsblk", []string{"--json", "--output", "NAME,TYPE,SIZE,FSTYPE,MOUNTPOINT,ROTA,MODEL", "--nodeps"}, ProfileLowRead},
		{"findmnt", []string{"--json", "--output", "TARGET,SOURCE,FSTYPE,OPTIONS"}, ProfileLowRead},
		{"pgrep", []string{"sshd"}, ProfileLowRead},
		{"free", []string{"-h"}, ProfileLowRead},
		{"uptime", []string{}, ProfileLowRead},
		{"hostname", []string{}, ProfilePublicRead},
		{"whoami", []string{}, ProfilePublicRead},
		{"date", []string{}, ProfilePublicRead},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			decision := policy.Evaluate(tt.command, tt.args, tt.profile)
			if !decision.Allowed {
				t.Fatalf("expected allowed for %s %v, got denied: %s", tt.command, tt.args, decision.Reason)
			}
			if decision.Profile != tt.profile {
				t.Fatalf("expected profile %s, got %s", tt.profile, decision.Profile)
			}
		})
	}
}

func TestExecPolicyAllowsSystemctlReadOnlyActions(t *testing.T) {
	policy := NewExecPolicy()

	allowed := []string{"is-active", "is-enabled", "is-failed", "status", "show", "list-timers", "list-sockets"}
	for _, action := range allowed {
		decision := policy.Evaluate("systemctl", []string{action, "sshd"}, ProfileLowRead)
		if !decision.Allowed {
			t.Fatalf("expected systemctl %s sshd to be allowed", action)
		}
	}

	// systemctl --version is a safe standalone action.
	decision := policy.Evaluate("systemctl", []string{"--version"}, ProfileLowRead)
	if !decision.Allowed {
		t.Fatalf("expected systemctl --version to be allowed")
	}
	decision = policy.Evaluate("systemctl", []string{"status", "sshd", "--no-pager", "-n", "20"}, ProfileLowRead)
	if !decision.Allowed {
		t.Fatalf("expected read-only systemctl status arguments to be allowed: %s", decision.Reason)
	}
	decision = policy.Evaluate("systemctl", []string{"list-units", "--type=service", "--all", "--no-pager", "--plain", "--legend=false"}, ProfileLowRead)
	if !decision.Allowed {
		t.Fatalf("expected read-only systemctl list-units arguments to be allowed: %s", decision.Reason)
	}
	decision = policy.Evaluate("systemctl", []string{"list-unit-files", "--type=service", "--no-pager", "--plain", "--legend=false"}, ProfileLowRead)
	if !decision.Allowed {
		t.Fatalf("expected read-only systemctl list-unit-files arguments to be allowed: %s", decision.Reason)
	}
}

func TestExecPolicyDeniesSystemctlMutations(t *testing.T) {
	policy := NewExecPolicy()
	for _, action := range []string{"start", "stop", "restart", "enable", "disable", "mask", "daemon-reload", "kill"} {
		decision := policy.Evaluate("systemctl", []string{action, "sshd"}, ProfileLowRead)
		if decision.Allowed {
			t.Fatalf("expected systemctl %s to be denied", action)
		}
		if !strings.Contains(decision.Reason, "not read-only") {
			t.Fatalf("unexpected denial for systemctl %s: %s", action, decision.Reason)
		}
	}
}

func TestExecPolicyRestrictsCatToApprovedSystemFacts(t *testing.T) {
	policy := NewExecPolicy()
	for _, path := range []string{"/etc/os-release", "/proc/loadavg", "/proc/meminfo", "/proc/uptime"} {
		if decision := policy.Evaluate("cat", []string{path}, ProfilePublicRead); !decision.Allowed {
			t.Fatalf("expected approved cat path %s: %s", path, decision.Reason)
		}
	}
	for _, path := range []string{"/etc/shadow", "/root/.ssh/id_rsa", "/etc/ssh/sshd_config", "/dev/zero"} {
		if decision := policy.Evaluate("cat", []string{path}, ProfileSensitiveRead); decision.Allowed {
			t.Fatalf("expected sensitive cat path %s to be denied", path)
		}
	}
}

func TestExecPolicyRestrictsLsofArguments(t *testing.T) {
	policy := NewExecPolicy()
	allowed := [][]string{
		{"-nP", "-F", "pcuftn", "--", "/var/log/messages"},
		{"-nP", "-F", "pcuftn", "--", "/tmp/demo.log"},
		{"-nP", "-F", "pcuftn", "-p", "123"},
	}
	for _, args := range allowed {
		if decision := policy.Evaluate("lsof", args, ProfileSensitiveRead); !decision.Allowed {
			t.Fatalf("expected lsof args %v to be allowed: %s", args, decision.Reason)
		}
	}
	denied := [][]string{
		{"/etc/shadow"},
		{"-nP", "-F", "pcuftn", "--", "/etc/shadow"},
		{"-nP", "-F", "pcuftn", "-p", "0"},
		{"-nP", "-F", "pcuftn", "-p", "1;id"},
	}
	for _, args := range denied {
		if decision := policy.Evaluate("lsof", args, ProfileSensitiveRead); decision.Allowed {
			t.Fatalf("expected lsof args %v to be denied", args)
		}
	}
}

func TestExecPolicyDeniesShellInterpreters(t *testing.T) {
	policy := NewExecPolicy()

	shells := []string{"sh", "bash", "zsh", "dash", "cmd", "powershell", "pwsh", "csh", "tcsh", "ksh"}
	for _, shell := range shells {
		decision := policy.Evaluate(shell, []string{"-c", "echo hello"}, ProfileLowRead)
		if decision.Allowed {
			t.Fatalf("expected shell %q to be denied", shell)
		}
		if !strings.Contains(decision.Reason, "shell interpreter") {
			t.Fatalf("expected shell-interpreter reason for %q, got: %s", shell, decision.Reason)
		}
		if decision.Profile != ProfileDenied {
			t.Fatalf("expected ProfileDenied for shell %q, got %s", shell, decision.Profile)
		}
	}
}

func TestExecPolicyDeniesPrivilegeEscalation(t *testing.T) {
	policy := NewExecPolicy()

	escalations := []string{"sudo", "su", "pkexec", "doas"}
	for _, cmd := range escalations {
		decision := policy.Evaluate(cmd, []string{"whoami"}, ProfileLowRead)
		if decision.Allowed {
			t.Fatalf("expected privilege escalation %q to be denied", cmd)
		}
		if !strings.Contains(decision.Reason, "privilege escalation") {
			t.Fatalf("expected privilege-escalation reason for %q, got: %s", cmd, decision.Reason)
		}
	}
}

func TestExecPolicyDeniesDangerousCommands(t *testing.T) {
	policy := NewExecPolicy()

	dangerous := []string{
		"rm", "mv", "cp", "chmod", "chown",
		"kill", "pkill", "killall",
		"mount", "umount", "mkfs", "dd", "tee",
		"iptables", "nft", "firewall-cmd",
		"reboot", "shutdown", "poweroff", "halt",
		"sed", "awk",
	}
	for _, cmd := range dangerous {
		decision := policy.Evaluate(cmd, []string{}, ProfileLowRead)
		if decision.Allowed {
			t.Fatalf("expected dangerous command %q to be denied", cmd)
		}
		if !strings.Contains(decision.Reason, "dangerous command") {
			t.Fatalf("expected dangerous-command reason for %q, got: %s", cmd, decision.Reason)
		}
	}
}

func TestExecPolicyDeniesEmptyCommand(t *testing.T) {
	policy := NewExecPolicy()
	decision := policy.Evaluate("", []string{}, ProfileLowRead)
	if decision.Allowed {
		t.Fatal("expected empty command to be denied")
	}
}

func TestExecPolicyDeniesNotInAllowlist(t *testing.T) {
	policy := NewExecPolicy()

	notAllowed := []string{"curl", "wget", "python", "ruby", "perl", "node", "gcc", "make"}
	for _, cmd := range notAllowed {
		decision := policy.Evaluate(cmd, []string{}, ProfileLowRead)
		if decision.Allowed {
			t.Fatalf("expected %q not in allowlist to be denied", cmd)
		}
		if !strings.Contains(decision.Reason, "not in the exec allowlist") {
			t.Fatalf("expected allowlist reason for %q, got: %s", cmd, decision.Reason)
		}
	}
}

func TestExecPolicyDeniesDeniedProfile(t *testing.T) {
	policy := NewExecPolicy()
	decision := policy.Evaluate("ps", []string{"aux"}, ProfileDenied)
	if decision.Allowed {
		t.Fatal("expected ProfileDenied to be denied")
	}
	if !strings.Contains(decision.Reason, "execution profile is denied") {
		t.Fatalf("expected profile-denied reason, got: %s", decision.Reason)
	}
}

func TestExecPolicyDeniesShellInjectionArgs(t *testing.T) {
	policy := NewExecPolicy()

	injections := []string{
		"hello; rm -rf /",
		"hello|cat /etc/passwd",
		"hello&",
		"hello$PATH",
		"hello`whoami`",
		"hello>file",
		"hello<file",
		"hello\nrm",
		"hello\rrm",
	}
	for _, arg := range injections {
		decision := policy.Evaluate("ps", []string{arg}, ProfileLowRead)
		if decision.Allowed {
			t.Fatalf("expected arg with shell chars %q to be denied", arg)
		}
		if !strings.Contains(decision.Reason, "forbidden shell characters") {
			t.Fatalf("expected shell-chars reason for %q, got: %s", arg, decision.Reason)
		}
	}
}

func TestExecPolicyNormalizesMixedCaseCommand(t *testing.T) {
	policy := NewExecPolicy()

	cases := []string{"PS", "Ps", "pS", "JOURNALCTL", "NetStat"}
	for _, cmd := range cases {
		decision := policy.Evaluate(cmd, []string{}, ProfileLowRead)
		if !decision.Allowed {
			t.Fatalf("expected mixed-case %q to be allowed after normalization", cmd)
		}
	}
	decision := policy.Evaluate("Systemctl", []string{"status", "sshd"}, ProfileLowRead)
	if !decision.Allowed {
		t.Fatalf("expected mixed-case Systemctl with read-only action to be allowed: %s", decision.Reason)
	}
}

func TestContainsShellCharsEdgeCases(t *testing.T) {
	if containsShellChars("normal-arg") {
		t.Fatal("normal-arg should not trigger shell chars")
	}
	if containsShellChars("file_name.txt") {
		t.Fatal("file_name.txt should not trigger shell chars")
	}
	if !containsShellChars("arg;cmd") {
		t.Fatal("arg;cmd should trigger shell chars")
	}
}

func TestIsSafeSystemctlArg(t *testing.T) {
	safe := []string{"is-active", "is-enabled", "is-failed", "status", "show", "list-units", "list-timers", "list-sockets", "list-unit-files", "--version", "--no-pager"}
	for _, arg := range safe {
		if !IsSafeSystemctlArg(arg) {
			t.Fatalf("expected %q to be safe systemctl arg", arg)
		}
	}

	dangerous := []string{"start", "stop", "restart", "enable", "disable", "mask", "unmask", "daemon-reload", "kill"}
	for _, arg := range dangerous {
		if IsSafeSystemctlArg(arg) {
			t.Fatalf("expected %q to NOT be safe systemctl arg", arg)
		}
	}
}

func TestCommandAllowlistContainsExpectedCommands(t *testing.T) {
	allowlist := commandAllowlist()
	expected := []string{"ps", "ss", "netstat", "journalctl", "df", "uname", "systemctl", "lsof", "lsblk", "findmnt", "rpm"}
	for _, cmd := range expected {
		if !allowlist[cmd] {
			t.Fatalf("expected %q in command allowlist", cmd)
		}
	}
}

func TestExecPolicyAllowsOnlyBoundedRPMVerify(t *testing.T) {
	policy := NewExecPolicy()
	if decision := policy.Evaluate("rpm", []string{"--verify", "openssh-server"}, ProfileSensitiveRead); !decision.Allowed {
		t.Fatalf("expected RPM verification to be allowed: %s", decision.Reason)
	}
	if decision := policy.Evaluate("rpm", []string{"-qa", "--qf", "%{NAME}\\t%{VERSION}\\t%{RELEASE}\\t%{ARCH}\\n"}, ProfileLowRead); !decision.Allowed {
		t.Fatalf("expected bounded RPM package inventory to be allowed: %s", decision.Reason)
	}
	for _, args := range [][]string{
		{"-Va"}, {"--verify", "--all"}, {"--erase", "openssh-server"}, {"--verify", "pkg;id"}, {"-qa"}, {"-qa", "--qf", "%{NAME}\n"},
	} {
		if decision := policy.Evaluate("rpm", args, ProfileSensitiveRead); decision.Allowed {
			t.Fatalf("expected RPM args %#v to be denied", args)
		}
	}
}

func TestExecPolicyAllowsOnlyBoundedInventoryCommands(t *testing.T) {
	policy := NewExecPolicy()
	if decision := policy.Evaluate("lsblk", []string{"--json", "--output", "NAME,TYPE,SIZE,FSTYPE,MOUNTPOINT,ROTA,MODEL", "--nodeps"}, ProfileLowRead); !decision.Allowed {
		t.Fatalf("expected bounded lsblk inventory to be allowed: %s", decision.Reason)
	}
	if decision := policy.Evaluate("findmnt", []string{"--json", "--output", "TARGET,SOURCE,FSTYPE,OPTIONS"}, ProfileLowRead); !decision.Allowed {
		t.Fatalf("expected bounded findmnt inventory to be allowed: %s", decision.Reason)
	}
	for _, tc := range []struct {
		command string
		args    []string
	}{
		{"lsblk", []string{"-f"}},
		{"lsblk", []string{"--json", "--output", "NAME,UUID", "--nodeps"}},
		{"findmnt", []string{}},
		{"findmnt", []string{"--json", "--output", "TARGET,SOURCE,OPTIONS,UUID"}},
		{"systemctl", []string{"list-units", "--type=mount"}},
	} {
		if decision := policy.Evaluate(tc.command, tc.args, ProfileLowRead); decision.Allowed {
			t.Fatalf("expected %s args %#v to be denied", tc.command, tc.args)
		}
	}
}
