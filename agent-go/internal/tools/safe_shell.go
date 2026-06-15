package tools

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

var errCommandNotAllowed = errors.New("command is not allowed by safe shell policy")

var safeCommandAllowlist = map[string][]string{
	"uname -a":            {"uname", "-a"},
	"hostname":            {"hostname"},
	"whoami":              {"whoami"},
	"date":                {"date"},
	"df -h":               {"df", "-h"},
	"free -h":             {"free", "-h"},
	"systemctl --version": {"systemctl", "--version"},
}

type SafeShellResult struct {
	Command    string `json:"command"`
	Allowed    bool   `json:"allowed"`
	ExitStatus string `json:"exit_status"`
	Output     string `json:"output"`
}

func SafeShell(ctx context.Context, input map[string]any) (any, string, string, error) {
	command := strings.Join(strings.Fields(stringValue(input, "command", "")), " ")
	result := SafeShellResult{
		Command: command,
		Allowed: false,
	}

	if command == "" || containsDangerousShellPattern(command) {
		result.ExitStatus = "blocked"
		return result, "blocked shell command", "high", errCommandNotAllowed
	}

	args, ok := safeCommandAllowlist[command]
	if !ok {
		result.ExitStatus = "blocked"
		return result, "command not in safe shell allowlist", "review", errCommandNotAllowed
	}

	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := exec.CommandContext(runCtx, args[0], args[1:]...).CombinedOutput()
	result.Allowed = true
	result.Output = strings.TrimSpace(string(output))
	if err != nil {
		result.ExitStatus = "failed"
		return result, "safe shell command failed", "review", err
	}

	result.ExitStatus = "completed"
	return result, "safe shell command executed", "review", nil
}

func containsDangerousShellPattern(command string) bool {
	lower := strings.ToLower(command)
	patterns := []string{
		"rm ",
		"rm -",
		"shutdown",
		"reboot",
		"mkfs",
		"dd ",
		"chmod 777",
		"curl ",
		"wget ",
		"| sh",
		"| bash",
		"format ",
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
