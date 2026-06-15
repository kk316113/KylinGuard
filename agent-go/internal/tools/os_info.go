package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type OSInfoResult struct {
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	Hostname  string    `json:"hostname"`
	Kernel    string    `json:"kernel,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func OSInfo(ctx context.Context, input map[string]any) (any, string, string, error) {
	_ = input

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	info := OSInfoResult{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Hostname:  hostname,
		Kernel:    kernelVersion(ctx),
		Timestamp: time.Now().UTC(),
	}

	return info, fmt.Sprintf("os=%s arch=%s host=%s", info.OS, info.Arch, info.Hostname), "low", nil
}

func kernelVersion(ctx context.Context) string {
	if runtime.GOOS == "windows" {
		return ""
	}

	cmd := exec.CommandContext(ctx, "uname", "-r")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
