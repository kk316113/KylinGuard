package execproxy

import (
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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

	if command == "systemctl" {
		if reason := validateSystemctlArgs(args); reason != "" {
			return denyDecision(reason)
		}
	}
	if command == "cat" {
		if reason := validateCatArgs(args); reason != "" {
			return denyDecision(reason)
		}
	}
	if command == "lsof" {
		if reason := validateLsofArgs(args); reason != "" {
			return denyDecision(reason)
		}
	}
	if command == "rpm" {
		if reason := validateRPMArgs(args); reason != "" {
			return denyDecision(reason)
		}
	}
	if command == "lsblk" {
		if reason := validateLSBLKArgs(args); reason != "" {
			return denyDecision(reason)
		}
	}
	if command == "findmnt" {
		if reason := validateFindmntArgs(args); reason != "" {
			return denyDecision(reason)
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
		"uptime":  true,
		"cat":     true,
		"lsof":    true,
		"rpm":     true,
		"lsblk":   true,
		"findmnt": true,
		"uname":   true, "hostname": true, "whoami": true, "date": true,
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
		"list-timers": true, "list-sockets": true, "list-unit-files": true,
		"--version": true, "--no-pager": true,
	}
	return allowed[arg]
}

var safeSystemdUnitPattern = regexp.MustCompile(`^[A-Za-z0-9_.@-]+$`)

func validateSystemctlArgs(args []string) string {
	if len(args) == 0 {
		return "systemctl requires an explicit read-only action"
	}
	if !IsSafeSystemctlArg(args[0]) || args[0] == "--no-pager" {
		return fmt.Sprintf("systemctl action %q is not read-only", args[0])
	}
	if args[0] == "--version" {
		if len(args) == 1 {
			return ""
		}
		return "systemctl --version does not accept additional arguments"
	}
	if args[0] == "list-units" || args[0] == "list-unit-files" {
		for _, arg := range args[1:] {
			switch arg {
			case "--no-pager", "--plain", "--all", "--legend=false", "--type=service":
				continue
			default:
				return fmt.Sprintf("systemctl list argument %q is not allowed", arg)
			}
		}
		return ""
	}

	expectLineCount := false
	for _, arg := range args[1:] {
		if expectLineCount {
			if !regexp.MustCompile(`^[0-9]{1,4}$`).MatchString(arg) {
				return "systemctl -n requires a numeric line count"
			}
			expectLineCount = false
			continue
		}
		switch arg {
		case "--no-pager":
			continue
		case "-n":
			expectLineCount = true
			continue
		}
		if !safeSystemdUnitPattern.MatchString(arg) {
			return fmt.Sprintf("systemctl argument %q is not a safe unit name", arg)
		}
	}
	if expectLineCount {
		return "systemctl -n requires a line count"
	}
	return ""
}

func validateCatArgs(args []string) string {
	if len(args) != 1 {
		return "cat requires exactly one approved read-only path"
	}
	allowedPaths := map[string]bool{
		"/etc/os-release": true,
		"/proc/loadavg":   true,
		"/proc/meminfo":   true,
		"/proc/uptime":    true,
	}
	if !allowedPaths[args[0]] {
		return fmt.Sprintf("cat path %q is not in the read allowlist", args[0])
	}
	return ""
}

func validateLsofArgs(args []string) string {
	if len(args) != 5 || args[0] != "-nP" || args[1] != "-F" || args[2] != "pcuftn" {
		return "lsof arguments must use the bounded KylinGuard field-output profile"
	}
	switch args[3] {
	case "-p":
		pid, err := strconv.Atoi(args[4])
		if err != nil || pid < 1 || pid > 4194304 {
			return "lsof pid must be between 1 and 4194304"
		}
		return ""
	case "--":
		path := filepath.ToSlash(filepath.Clean(args[4]))
		for _, prefix := range []string{"/var/log/", "/tmp/", "/var/tmp/", "/opt/kylin-guard/"} {
			if strings.HasPrefix(path, prefix) {
				return ""
			}
		}
		return fmt.Sprintf("lsof path %q is not in the inspection allowlist", path)
	default:
		return "lsof requires either an approved path or a numeric pid"
	}
}

var safeRPMPackagePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+:-]{0,127}$`)

func validateRPMArgs(args []string) string {
	if len(args) == 2 && args[0] == "--verify" {
		if !safeRPMPackagePattern.MatchString(args[1]) {
			return "rpm package name contains unsafe characters"
		}
		return ""
	}
	if len(args) == 3 && args[0] == "-qa" && args[1] == "--qf" && args[2] == "%{NAME}\\t%{VERSION}\\t%{RELEASE}\\t%{ARCH}\\n" {
		return ""
	}
	return "rpm is restricted to read-only verification or package inventory query"
}

func validateLSBLKArgs(args []string) string {
	if len(args) != 4 {
		return "lsblk requires the bounded JSON output profile"
	}
	if args[0] != "--json" || args[1] != "--output" || args[3] != "--nodeps" {
		return "lsblk arguments must use --json --output <columns> --nodeps"
	}
	if args[2] != "NAME,TYPE,SIZE,FSTYPE,MOUNTPOINT,ROTA,MODEL" {
		return "lsblk output columns are not in the allowlist"
	}
	return ""
}

func validateFindmntArgs(args []string) string {
	if len(args) != 3 {
		return "findmnt requires the bounded JSON output profile"
	}
	if args[0] != "--json" || args[1] != "--output" {
		return "findmnt arguments must use --json --output <columns>"
	}
	if args[2] != "TARGET,SOURCE,FSTYPE,OPTIONS" {
		return "findmnt output columns are not in the allowlist"
	}
	return ""
}

// Platform returns the current OS platform string.
func platform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}
