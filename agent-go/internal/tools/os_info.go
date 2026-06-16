package tools

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"kylin-guard-agent/agent-go/internal/execproxy"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

type OSInfoResult struct {
	OS               string                     `json:"os"`
	Arch             string                     `json:"arch"`
	Hostname         string                     `json:"hostname"`
	Kernel           string                     `json:"kernel,omitempty"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
}

func OSInfo(ctx context.Context, input map[string]any) (any, string, string, error) {
	_ = input

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	kernel := ""
	if runtime.GOOS != "windows" {
		exec := execproxy.NewExecutor()
		result, execErr := exec.Execute(ctx, execproxy.CommandSpec{
			ToolName: "os_info",
			Profile:  execproxy.ProfilePublicRead,
			Command:  "uname",
			Args:     []string{"-r"},
			Reason:   "collect kernel version",
		})
		if execErr == nil && result.Status == "ok" {
			kernel = trim(result.Stdout)
		}
	}

	nec := execproxy.NativeExecutionContext(execproxy.ProfilePublicRead, "native_go+runtime", "public OS info collection")
	info := OSInfoResult{
		OS:               runtime.GOOS,
		Arch:             runtime.GOARCH,
		Hostname:         hostname,
		Kernel:           kernel,
		Timestamp:        time.Now().UTC(),
		ExecutionContext: ecPtr(nec),
	}

	return info, fmt.Sprintf("os=%s arch=%s host=%s", info.OS, info.Arch, info.Hostname), "low", nil
}

func trim(s string) string {
	i := len(s) - 1
	for i >= 0 && (s[i] == '\n' || s[i] == '\r') {
		i--
	}
	return s[:i+1]
}
