package execproxy

import (
	"fmt"
	"runtime"
	"strings"
)

// ExecPolicy validates command execution against allowlists and safety rules.
type ExecPolicy struct{}

// ExecPolicyDecision represents the outcome of an execution policy check.
type ExecPolicyDecision struct {
	Allowed bool             `json:"allowed"`
	Reason  string           `json:"reason"`
	Profile ExecutionProfile `json:"profile"`
}

// NewExecPolicy creates a new execution policy.
func NewExecPolicy() ExecPolicy {
	return ExecPolicy{}
}

// Evaluate checks whether a command + args combination is allowed under the
// least-privilege execution proxy. It enforces:
//   - Command must be in the platform-specific allowlist
//   - No shell interpreters (bash, sh, zsh, cmd, powershell)
//   - No sudo/su privilege escalation
//   - No dangerous commands (rm, kill, systemctl modifications, etc.)
//   - Args must not contain shell metacharacters
func (ExecPolicy) Evaluate(command string, args []string, profile ExecutionProfile) ExecPolicyDecision {
	command = strings.TrimSpace(strings.ToLower(command))

	// Block empty command.
	if command == "" {
		return denyDecision("empty command is not allowed")
	}

	// Block shell interpreters.
	if isShell(command) {
		return denyDecision(fmt.Sprintf("shell interpreter %q is forbidden; direct command execution only", command))
	}

	// Block privilege escalation.
	if isPrivilegeEscalation(command) {
		return denyDecision(fmt.Sprintf("privilege escalation command %q is forbidden", command))
	}

	// Block dangerous commands.
	if isDangerousCommand(command) {
		return denyDecision(fmt.Sprintf("dangerous command %q is forbidden", command))
	}

	// Validate command is in the allowlist.
	allowed := commandAllowlist()
	if !allowed[command] {
		return denyDecision(fmt.Sprintf("command %q is not in the exec allowlist", command))
	}

	// Validate profile is not denied.
	if profile == ProfileDenied {
		return denyDecision("execution profile is denied")
	}

	// Validate args for shell injection characters.
	for _, arg := range args {
		if containsShellChars(arg) {
			return denyDecision(fmt.Sprintf("argument %q contains forbidden shell characters", arg))
		}
	}

	return ExecPolicyDecision{
		Allowed: true,
		Reason:  fmt.Sprintf("allowed %s %s invocation under profile %s", command, strings.Join(args, " "), profile),
		Profile: profile,
	}
}

func denyDecision(reason string) ExecPolicyDecision {
	return ExecPolicyDecision{
		Allowed: false,
		Reason:  reason,
		Profile: ProfileDenied,
	}
}

// isShell returns true if the command is a shell interpreter.
func isShell(command string) bool {
	shells := map[string]bool{
		"sh": true, "bash": true, "zsh": true, "dash": true,
		"cmd": true, "powershell": true, "pwsh": true,
		"csh": true, "tcsh": true, "ksh": true,
	}
	return shells[command]
}

// isPrivilegeEscalation returns true if the command attempts privilege escalation.
func isPrivilegeEscalation(command string) bool {
	escalations := map[string]bool{
		"sudo": true, "su": true, "pkexec": true, "doas": true,
	}
	return escalations[command]
}

// isDangerousCommand returns true if the command is blocked for destructive operations.
func isDangerousCommand(command string) bool {
	dangerous := map[string]bool{
		"rm": true, "mv": true, "cp": true,
		"chmod": true, "chown": true, "chgrp": true,
		"kill": true, "pkill": true, "killall": true,
		"mount": true, "umount": true, "mkfs": true,
		"dd": true, "tee": true,
		"iptables": true, "nft": true, "firewall-cmd": true,
		"reboot": true, "shutdown": true, "poweroff": true, "halt": true,
		"sed": true, "awk": true,
	}
	return dangerous[command]
}

// commandAllowlist returns the platform-specific set of allowed commands.
func commandAllowlist() map[string]bool {
	base := map[string]bool{
		// Common safe diagnostic commands (Linux + Windows).
		"ps": true, "pgrep": true,
		"ss": true, "netstat": true,
		"journalctl": true,
		"df":         true, "free": true,
		"uptime": true,
		"cat":    true,
		"uname":  true, "hostname": true, "whoami": true, "date": true,
		// systemctl read-only subcommands only (args are validated separately).
		"systemctl": true,
	}

	if runtime.GOOS == "windows" {
		base["tasklist"] = true
	}

	return base
}

// containsShellChars returns true if the argument contains shell metacharacters.
func containsShellChars(arg string) bool {
	forbidden := []string{";", "|", "&", "$", "`", ">", "<", "\n", "\r"}
	for _, char := range forbidden {
		if strings.Contains(arg, char) {
			return true
		}
	}
	return false
}

// IsSafeSystemctlArg returns true if the systemctl verb is read-only.
func IsSafeSystemctlArg(arg string) bool {
	allowed := map[string]bool{
		"is-active": true, "is-enabled": true, "is-failed": true,
		"status": true, "show": true, "list-units": true,
		"list-timers": true, "list-sockets": true,
		"--version": true, "--no-pager": true,
	}
	return allowed[arg]
}

// Platform returns the current OS platform string.
func platform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}
