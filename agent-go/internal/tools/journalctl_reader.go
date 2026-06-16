package tools

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// JournalctlReaderResult holds the output of journalctl log reading.
type JournalctlReaderResult struct {
	ServiceName string    `json:"service_name"`
	Lines       []string  `json:"lines"`
	Source      string    `json:"source"`
	Status      string    `json:"status"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

// safeServiceNamePattern allows letters, digits, underscore, hyphen, dot, and @ (for systemd instance units).
var safeJournalServicePattern = regexp.MustCompile(`^[A-Za-z0-9_.@-]+$`)

// SafeJournalServiceName checks whether a systemd service name conforms to the safe character set.
func SafeJournalServiceName(name string) bool {
	return name != "" && safeJournalServicePattern.MatchString(name)
}

// JournalctlReader reads recent systemd journal logs for a specified service.
// It is a read-only diagnostic tool and does not modify system state or journal files.
func JournalctlReader(ctx context.Context, input map[string]any) (any, string, string, error) {
	serviceName := strings.TrimSpace(stringValue(input, "service_name", "sshd"))
	lines := intValue(input, "lines", 100)
	if lines < 1 {
		lines = 1
	}
	if lines > 500 {
		lines = 500
	}

	now := time.Now().UTC()

	// Validate service_name against shell injection.
	if serviceName == "" || !safeJournalServicePattern.MatchString(serviceName) {
		result := JournalctlReaderResult{
			ServiceName: serviceName,
			Lines:       []string{},
			Source:      "",
			Status:      "denied",
			Message:     fmt.Sprintf("invalid service_name %q: only letters, digits, underscore, hyphen, dot, and @ are allowed", serviceName),
			Timestamp:   now,
		}
		return result, "journalctl_reader: invalid service_name rejected", "low", fmt.Errorf("invalid service_name %q", serviceName)
	}

	if runtime.GOOS != "linux" {
		result := JournalctlReaderResult{
			ServiceName: serviceName,
			Lines:       []string{},
			Source:      "journalctl",
			Status:      "unsupported",
			Message:     "journalctl is only available on Linux with systemd",
			Timestamp:   now,
		}
		return result, "journalctl_reader unsupported on non-Linux host", "low", nil
	}

	_, err := exec.LookPath("journalctl")
	if err != nil {
		result := JournalctlReaderResult{
			ServiceName: serviceName,
			Lines:       []string{},
			Source:      "journalctl",
			Status:      "error",
			Message:     "journalctl command not found; systemd-journald may not be installed",
			Timestamp:   now,
		}
		return result, "journalctl_reader: journalctl not found", "review", err
	}

	// Build safe args: journalctl -u <service_name> -n <lines> --no-pager
	args := []string{"-u", serviceName, "-n", fmt.Sprintf("%d", lines), "--no-pager"}
	output, cmdErr := runCommand(ctx, defaultCommandTimeout, "journalctl", args...)

	status := "ok"
	message := "journalctl log read completed"
	summary := fmt.Sprintf("journalctl_reader read %d lines from %s", lines, serviceName)
	riskHint := "low"

	if cmdErr != nil {
		// journalctl may return non-zero when service has no logs or insufficient permissions.
		if strings.Contains(cmdErr.Error(), "permission") || strings.Contains(cmdErr.Error(), "denied") {
			status = "error"
			message = fmt.Sprintf("journalctl permission denied: %v", cmdErr)
			summary = fmt.Sprintf("journalctl_reader: permission denied for %s", serviceName)
			riskHint = "review"
		} else {
			status = "warning"
			message = fmt.Sprintf("journalctl returned with error (service may have no logs): %v", cmdErr)
			summary = fmt.Sprintf("journalctl_reader: %s returned with non-zero status", serviceName)
		}
	}

	logLines := strings.Split(strings.TrimSpace(output.Stdout), "\n")
	if len(logLines) == 1 && logLines[0] == "" {
		logLines = []string{}
	}

	result := JournalctlReaderResult{
		ServiceName: serviceName,
		Lines:       logLines,
		Source:      "journalctl",
		Status:      status,
		Message:     message,
		Timestamp:   now,
	}
	return result, summary, riskHint, nil
}
