package tools

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/execproxy"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

type DiskMemoryCheckerResult struct {
	Filesystems      []FilesystemInfo           `json:"filesystems"`
	Memory           *MemInfo                   `json:"memory"`
	RiskLevel        string                     `json:"risk_level"`
	Source           string                     `json:"source"`
	Message          string                     `json:"message"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
}

type FilesystemInfo struct {
	Filesystem  string  `json:"filesystem"`
	Mountpoint  string  `json:"mountpoint"`
	Size        string  `json:"size,omitempty"`
	Used        string  `json:"used,omitempty"`
	Avail       string  `json:"avail,omitempty"`
	UsedPercent float64 `json:"used_percent"`
}

func DiskMemoryChecker(ctx context.Context, input map[string]any) (any, string, string, error) {
	includeTmpfs := false
	if val, ok := input["include_tmpfs"]; ok {
		if boolVal, ok := val.(bool); ok {
			includeTmpfs = boolVal
		}
	}
	now := time.Now().UTC()

	if runtime.GOOS != "linux" {
		mem, _ := readMemInfo()
		nec := execproxy.NativeExecutionContext(execproxy.ProfileLowRead, "unsupported_platform", "disk_memory_checker partial on non-Linux")
		result := DiskMemoryCheckerResult{
			Filesystems:      []FilesystemInfo{},
			Memory:           mem,
			RiskLevel:        "unknown",
			Source:           "",
			Message:          "disk_memory_checker filesystem inspection is only supported on Linux",
			Timestamp:        now,
			ExecutionContext: ecPtr(nec),
		}
		return result, "disk_memory_checker filesystem inspection unsupported on non-Linux host", "low", nil
	}

	var warnings []string

	exec := execproxy.NewExecutor()
	dfResult, dfErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName: "disk_memory_checker",
		Profile:  execproxy.ProfileLowRead,
		Command:  "df",
		Args:     []string{"-h"},
		Reason:   "inspect disk usage",
	})

	var filesystems []FilesystemInfo
	var execCtx execproxy.ExecutionContext
	if dfErr == nil && dfResult.Status == "ok" {
		filesystems, _ = parseDFOutput(dfResult.Stdout, includeTmpfs)
		execCtx = dfResult.Context
	} else {
		filesystems, _ = readMountsFromProc(includeTmpfs)
		execCtx = execproxy.NativeExecutionContext(execproxy.ProfileLowRead, "procfs:/proc/mounts", "df not available, reading /proc/mounts")
		if dfErr != nil {
			warnings = append(warnings, fmt.Sprintf("df failed: %v", dfErr))
		}
	}

	mem, memWarn := readMemInfo()
	if memWarn != "" {
		warnings = append(warnings, memWarn)
	}

	riskLevel := "low"
	for _, fs := range filesystems {
		if fs.UsedPercent >= 90 {
			riskLevel = "high"
			break
		}
		if fs.UsedPercent >= 80 && riskLevel == "low" {
			riskLevel = "medium"
		}
	}
	if mem != nil {
		if mem.MemAvailableRatio < 0.1 {
			riskLevel = "high"
		} else if mem.MemAvailableRatio < 0.2 && riskLevel == "low" {
			riskLevel = "medium"
		}
	}

	message := "disk and memory inspection completed"
	if len(warnings) > 0 {
		message = fmt.Sprintf("disk and memory inspection completed with warnings: %s", strings.Join(warnings, "; "))
	}

	summary := fmt.Sprintf("disk_memory_checker inspected %d filesystems, risk=%s", len(filesystems), riskLevel)

	result := DiskMemoryCheckerResult{
		Filesystems:      filesystems,
		Memory:           mem,
		RiskLevel:        riskLevel,
		Source:           "df,procfs",
		Message:          message,
		Timestamp:        now,
		ExecutionContext: ecPtr(execCtx),
	}
	return result, summary, riskLevel, nil
}

func parseDFOutput(output string, includeTmpfs bool) ([]FilesystemInfo, error) {
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("no df output")
	}
	var filesystems []FilesystemInfo
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		fs := fields[0]
		mount := fields[5]
		size := fields[1]
		used := fields[2]
		avail := fields[3]
		percentStr := strings.TrimSuffix(fields[4], "%")
		if !includeTmpfs && (strings.HasPrefix(fs, "tmpfs") || strings.HasPrefix(fs, "devtmpfs") || fs == "none") {
			continue
		}
		percent, err := strconv.ParseFloat(percentStr, 64)
		if err != nil {
			percent = 0
		}
		filesystems = append(filesystems, FilesystemInfo{Filesystem: fs, Mountpoint: mount, Size: size, Used: used, Avail: avail, UsedPercent: percent})
	}
	return filesystems, nil
}

func readMountsFromProc(includeTmpfs bool) ([]FilesystemInfo, string) {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil, fmt.Sprintf("cannot read /proc/mounts: %v", err)
	}
	var filesystems []FilesystemInfo
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		fs := fields[0]
		mount := fields[1]
		fsType := fields[2]
		if !includeTmpfs && (fsType == "tmpfs" || fsType == "devtmpfs") {
			continue
		}
		filesystems = append(filesystems, FilesystemInfo{Filesystem: fs, Mountpoint: mount})
	}
	return filesystems, ""
}
