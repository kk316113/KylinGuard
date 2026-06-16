package tools

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/execproxy"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

type ServiceStatusResult struct {
	Service          string                     `json:"service"`
	Status           string                     `json:"status"`
	Enabled          string                     `json:"enabled"`
	SystemdAvailable bool                       `json:"systemd_available"`
	StatusOutput     string                     `json:"status_output"`
	Message          string                     `json:"message"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
}

func ServiceStatus(ctx context.Context, input map[string]any) (any, string, string, error) {
	service := svcNameFromInput(input)
	now := time.Now().UTC()

	if runtime.GOOS != "linux" {
		nec := execproxy.NativeExecutionContext(execproxy.ProfileLowRead, "unsupported_platform", "service_status unsupported on non-Linux")
		result := ServiceStatusResult{
			Service:          service,
			Status:           "unsupported",
			Enabled:          "unknown",
			SystemdAvailable: false,
			Message:          "systemctl status checks are only supported on Linux targets",
			Timestamp:        now,
			ExecutionContext: ecPtr(nec),
		}
		return result, "service_status unsupported on non-Linux host", "low", nil
	}

	exec := execproxy.NewExecutor()

	activeResult, activeErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName:         "service_status",
		Profile:          execproxy.ProfileLowRead,
		Command:          "systemctl",
		Args:             []string{"is-active", service},
		AllowNonZeroExit: true,
		Reason:           "check service active status",
	})
	activeStatus := svcFirstNonEmpty(strings.TrimSpace(activeResult.Stdout), "unknown")
	if activeErr != nil && activeStatus == "unknown" {
		activeStatus = svcSummarize(activeErr)
	}

	enabledResult, enabledErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName:         "service_status",
		Profile:          execproxy.ProfileLowRead,
		Command:          "systemctl",
		Args:             []string{"is-enabled", service},
		AllowNonZeroExit: true,
		Reason:           "check service enabled status",
	})
	enabledStatus := svcFirstNonEmpty(strings.TrimSpace(enabledResult.Stdout), "unknown")
	if enabledErr != nil && enabledStatus == "unknown" {
		enabledStatus = svcSummarize(enabledErr)
	}

	statusResult, statusErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName:         "service_status",
		Profile:          execproxy.ProfileLowRead,
		Command:          "systemctl",
		Args:             []string{"status", service, "--no-pager", "-n", "20"},
		AllowNonZeroExit: true,
		Reason:           "check service detailed status",
	})
	statusText := strings.TrimSpace(statusResult.Stdout)
	if strings.TrimSpace(statusResult.Stderr) != "" {
		statusText = strings.TrimSpace(statusText + "\n" + statusResult.Stderr)
	}

	message := "systemctl service status probe completed"
	summary := fmt.Sprintf("service %s status=%s enabled=%s", service, activeStatus, enabledStatus)
	if activeErr != nil || enabledErr != nil || statusErr != nil {
		message = "systemctl service status probe completed with command errors"
	}

	result := ServiceStatusResult{
		Service:          service,
		Status:           activeStatus,
		Enabled:          enabledStatus,
		SystemdAvailable: true,
		StatusOutput:     statusText,
		Message:          message,
		Timestamp:        now,
		ExecutionContext: ecPtr(statusResult.Context),
	}
	return result, summary, "low", nil
}

func svcNameFromInput(input map[string]any) string {
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

func svcFirstNonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func serviceNameFromInput(input map[string]any) string {
	return svcNameFromInput(input)
}

func svcSummarize(err error) string {
	if err == nil {
		return "unknown"
	}
	text := err.Error()
	if len(text) > 160 {
		return text[:160]
	}
	return text
}

func summarizeCommandError(err error) string {
	return svcSummarize(err)
}
