package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DiskMemoryCheckerResult holds the output of disk and memory inspection.
type DiskMemoryCheckerResult struct {
	Filesystems []FilesystemInfo `json:"filesystems"`
	Memory      *MemInfo         `json:"memory"`
	RiskLevel   string           `json:"risk_level"`
	Source      string           `json:"source"`
	Message     string           `json:"message"`
	Timestamp   time.Time        `json:"timestamp"`
}

// FilesystemInfo describes a single mounted filesystem.
type FilesystemInfo struct {
	Filesystem  string  `json:"filesystem"`
	Mountpoint  string  `json:"mountpoint"`
	Size        string  `json:"size,omitempty"`
	Used        string  `json:"used,omitempty"`
	Avail       string  `json:"avail,omitempty"`
	UsedPercent float64 `json:"used_percent"`
}

// DiskMemoryChecker inspects disk usage and memory summary.
// It is a read-only diagnostic tool and does not modify disks or mounts.
func DiskMemoryChecker(ctx context.Context, input map[string]any) (any, string, string, error) {
	includeTmpfs := false
	if val, ok := input["include_tmpfs"]; ok {
		if boolVal, ok := val.(bool); ok {
			includeTmpfs = boolVal
		}
	}

	now := time.Now().UTC()

	if runtime.GOOS != "linux" {
		// On non-Linux, still try to read /proc/meminfo for memory.
		mem, _ := readMemInfo()
		result := DiskMemoryCheckerResult{
			Filesystems: []FilesystemInfo{},
			Memory:      mem,
			RiskLevel:   "unknown",
			Source:      "",
			Message:     "disk_memory_checker filesystem inspection is only supported on Linux",
			Timestamp:   now,
		}
		return result, "disk_memory_checker filesystem inspection unsupported on non-Linux host", "low", nil
	}

	var warnings []string

	filesystems, fsWarn := readFilesystems(ctx, includeTmpfs)
	if fsWarn != "" {
		warnings = append(warnings, fsWarn)
	}

	mem, memWarn := readMemInfo()
	if memWarn != "" {
		warnings = append(warnings, memWarn)
	}

	// Determine risk level from disk usage.
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
	// Also check memory.
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

	diskCount := len(filesystems)
	summary := fmt.Sprintf("disk_memory_checker inspected %d filesystems, risk=%s", diskCount, riskLevel)

	result := DiskMemoryCheckerResult{
		Filesystems: filesystems,
		Memory:      mem,
		RiskLevel:   riskLevel,
		Source:      "df,procfs",
		Message:     message,
		Timestamp:   now,
	}
	return result, summary, riskLevel, nil
}

func readFilesystems(ctx context.Context, includeTmpfs bool) ([]FilesystemInfo, string) {
	_, err := exec.LookPath("df")
	if err != nil {
		return nil, "df command not found"
	}

	args := []string{"-h"}
	output, cmdErr := runCommand(ctx, defaultCommandTimeout, "df", args...)
	if cmdErr != nil {
		return nil, fmt.Sprintf("df -h failed: %v", cmdErr)
	}

	// If df failed, try reading /proc/mounts and use statfs.
	filesystems, parseErr := parseDFOutput(output.Stdout, includeTmpfs)
	if parseErr != nil {
		// Fallback: try reading /proc/mounts.
		return readMountsFromProc(includeTmpfs)
	}
	return filesystems, ""
}

func parseDFOutput(output string, includeTmpfs bool) ([]FilesystemInfo, error) {
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("no df output")
	}

	// Skip header line.
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

		// Filter tmpfs if not requested.
		if !includeTmpfs && (strings.HasPrefix(fs, "tmpfs") || strings.HasPrefix(fs, "devtmpfs") || fs == "none") {
			continue
		}

		percent, err := strconv.ParseFloat(percentStr, 64)
		if err != nil {
			percent = 0
		}

		filesystems = append(filesystems, FilesystemInfo{
			Filesystem:  fs,
			Mountpoint:  mount,
			Size:        size,
			Used:        used,
			Avail:       avail,
			UsedPercent: percent,
		})
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

		filesystems = append(filesystems, FilesystemInfo{
			Filesystem: fs,
			Mountpoint: mount,
		})
	}
	return filesystems, ""
}
