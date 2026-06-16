package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type ServiceStatusResult struct {
	Service          string    `json:"service"`
	Status           string    `json:"status"`
	Enabled          string    `json:"enabled"`
	SystemdAvailable bool      `json:"systemd_available"`
	StatusOutput     string    `json:"status_output"`
	Message          string    `json:"message"`
	Timestamp        time.Time `json:"timestamp"`
}

func ServiceStatus(ctx context.Context, input map[string]any) (any, string, string, error) {
	service := serviceNameFromInput(input)
	now := time.Now().UTC()

	if runtime.GOOS != "linux" {
		result := ServiceStatusResult{
			Service:          service,
			Status:           "unsupported",
			Enabled:          "unknown",
			SystemdAvailable: false,
			Message:          "systemctl status checks are only supported on Linux targets",
			Timestamp:        now,
		}
		return result, "service_status unsupported on non-Linux host", "low", nil
	}

	if _, err := exec.LookPath("systemctl"); err != nil {
		result := ServiceStatusResult{
			Service:          service,
			Status:           "error",
			Enabled:          "unknown",
			SystemdAvailable: false,
			Message:          "systemctl not found",
			Timestamp:        now,
		}
		return result, "systemctl not found", "review", err
	}

	activeOutput, activeErr := runCommand(ctx, defaultCommandTimeout, "systemctl", "is-active", service)
	enabledOutput, enabledErr := runCommand(ctx, defaultCommandTimeout, "systemctl", "is-enabled", service)
	statusOutput, statusErr := runCommand(ctx, defaultCommandTimeout, "systemctl", "status", service, "--no-pager", "-n", "20")

	activeStatus := firstNonEmpty(strings.TrimSpace(activeOutput.Stdout), "unknown")
	if activeErr != nil && activeStatus == "unknown" {
		activeStatus = summarizeCommandError(activeErr)
	}
	enabledStatus := firstNonEmpty(strings.TrimSpace(enabledOutput.Stdout), "unknown")
	if enabledErr != nil && enabledStatus == "unknown" {
		enabledStatus = summarizeCommandError(enabledErr)
	}

	statusText := strings.TrimSpace(statusOutput.Combined())
	message := "systemctl service status probe completed"
	summary := fmt.Sprintf("service %s status=%s enabled=%s", service, activeStatus, enabledStatus)
	if activeErr != nil || enabledErr != nil || statusErr != nil {
		message = "systemctl service status probe completed with command errors"
		summary = fmt.Sprintf("%s; some systemctl probes returned non-zero status", summary)
	}

	result := ServiceStatusResult{
		Service:          service,
		Status:           activeStatus,
		Enabled:          enabledStatus,
		SystemdAvailable: true,
		StatusOutput:     statusText,
		Message:          message,
		Timestamp:        now,
	}
	return result, summary, "low", nil
}

func serviceNameFromInput(input map[string]any) string {
	service := stringValue(input, "service_name", "")
	if service == "" {
		service = stringValue(input, "service", "sshd")
	}
	service = strings.TrimSpace(service)
	if service == "" {
		return "sshd"
	}
	return service
}

func firstNonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func summarizeCommandError(err error) string {
	if err == nil {
		return "unknown"
	}
	text := err.Error()
	if len(text) > 160 {
		return text[:160]
	}
	return text
}
