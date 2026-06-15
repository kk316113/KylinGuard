package tools

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type ServiceStatusResult struct {
	Service          string    `json:"service"`
	Status           string    `json:"status"`
	SystemdAvailable bool      `json:"systemd_available"`
	Message          string    `json:"message"`
	Timestamp        time.Time `json:"timestamp"`
}

func ServiceStatus(ctx context.Context, input map[string]any) (any, string, string, error) {
	service := stringValue(input, "service", "system")
	now := time.Now().UTC()

	if runtime.GOOS != "linux" {
		result := ServiceStatusResult{
			Service:          service,
			Status:           "not_supported",
			SystemdAvailable: false,
			Message:          "systemctl is only checked on Linux targets",
			Timestamp:        now,
		}
		return result, "service status check skipped on non-Linux host", "low", nil
	}

	if _, err := exec.LookPath("systemctl"); err != nil {
		result := ServiceStatusResult{
			Service:          service,
			Status:           "not_found",
			SystemdAvailable: false,
			Message:          "systemctl not found",
			Timestamp:        now,
		}
		return result, "systemctl not found", "low", nil
	}

	args := []string{"is-system-running"}
	if service != "" && service != "system" {
		args = []string{"is-active", service}
	}

	output, err := exec.CommandContext(ctx, "systemctl", args...).CombinedOutput()
	status := strings.TrimSpace(string(output))
	if status == "" && err != nil {
		status = err.Error()
	}

	result := ServiceStatusResult{
		Service:          service,
		Status:           status,
		SystemdAvailable: true,
		Message:          "systemctl status probe completed",
		Timestamp:        now,
	}
	return result, "service status checked", "low", nil
}
