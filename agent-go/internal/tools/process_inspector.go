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

// ProcessInspectorResult holds the output of process inspection.
type ProcessInspectorResult struct {
	Processes []ProcessInfo `json:"processes"`
	Count     int           `json:"count"`
	Source    string        `json:"source"`
	Message   string        `json:"message"`
	Timestamp time.Time     `json:"timestamp"`
}

// ProcessInfo describes a single process.
type ProcessInfo struct {
	PID   int    `json:"pid"`
	Name  string `json:"name"`
	User  string `json:"user,omitempty"`
	State string `json:"state,omitempty"`
	Cmd   string `json:"cmd,omitempty"`
}

// safeProcessNamePattern allows letters, digits, underscore, hyphen, and dot.
var safeProcessNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// SafeProcessName checks whether a process name conforms to the safe character set.
func SafeProcessName(name string) bool {
	return name != "" && safeProcessNamePattern.MatchString(name)
}

// ProcessInspector inspects running processes, optionally filtered by name.
// It is a read-only diagnostic tool and does not modify or kill processes.
func ProcessInspector(ctx context.Context, input map[string]any) (any, string, string, error) {
	name := strings.TrimSpace(stringValue(input, "name", ""))
	limit := intValue(input, "limit", 20)
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	now := time.Now().UTC()

	// Validate process name if provided.
	if name != "" && !safeProcessNamePattern.MatchString(name) {
		result := ProcessInspectorResult{
			Processes: []ProcessInfo{},
			Count:     0,
			Source:    "",
			Message:   fmt.Sprintf("invalid process name %q: only letters, digits, underscore, hyphen, and dot are allowed", name),
			Timestamp: now,
		}
		return result, "process_inspector: invalid process name rejected", "low", fmt.Errorf("invalid process name %q", name)
	}

	switch runtime.GOOS {
	case "linux":
		return processInspectorLinux(ctx, name, limit, now)
	case "windows":
		return processInspectorWindows(ctx, name, limit, now)
	default:
		result := ProcessInspectorResult{
			Processes: []ProcessInfo{},
			Count:     0,
			Source:    "",
			Message:   fmt.Sprintf("process_inspector is not supported on %s", runtime.GOOS),
			Timestamp: now,
		}
		return result, "process_inspector unsupported on this OS", "low", nil
	}
}

func processInspectorLinux(ctx context.Context, name string, limit int, now time.Time) (any, string, string, error) {
	_, err := exec.LookPath("ps")
	if err != nil {
		result := ProcessInspectorResult{
			Processes: []ProcessInfo{},
			Count:     0,
			Source:    "",
			Message:   "ps command not found",
			Timestamp: now,
		}
		return result, "process_inspector: ps not found", "low", err
	}

	args := []string{"aux"}
	output, cmdErr := runCommand(ctx, defaultCommandTimeout, "ps", args...)

	if cmdErr != nil {
		result := ProcessInspectorResult{
			Processes: []ProcessInfo{},
			Count:     0,
			Source:    "ps",
			Message:   fmt.Sprintf("ps aux failed: %v", cmdErr),
			Timestamp: now,
		}
		return result, "process_inspector: ps aux failed", "review", cmdErr
	}

	lines := strings.Split(output.Stdout, "\n")
	// Skip the header line if present.
	startIdx := 0
	if len(lines) > 0 && strings.HasPrefix(strings.ToLower(lines[0]), "user") {
		startIdx = 1
	}

	processes := make([]ProcessInfo, 0, limit)
	for _, line := range lines[startIdx:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		// ps aux format: USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND
		procName := fields[10]
		if len(fields) > 11 {
			procName = fields[10]
		}

		// Filter by name if specified.
		if name != "" {
			baseName := procName
			if idx := strings.LastIndex(baseName, "/"); idx >= 0 {
				baseName = baseName[idx+1:]
			}
			if !strings.Contains(baseName, name) {
				continue
			}
		}

		pid := 0
		if len(fields) > 1 {
			fmt.Sscanf(fields[1], "%d", &pid)
		}

		user := ""
		if len(fields) > 0 {
			user = fields[0]
		}

		state := ""
		if len(fields) > 7 {
			state = fields[7]
		}

		cmd := procName
		if len(fields) > 10 {
			cmd = strings.Join(fields[10:], " ")
		}

		processes = append(processes, ProcessInfo{
			PID:   pid,
			Name:  procName,
			User:  user,
			State: state,
			Cmd:   cmd,
		})

		if len(processes) >= limit {
			break
		}
	}

	summary := fmt.Sprintf("process_inspector found %d processes", len(processes))
	if name != "" {
		summary = fmt.Sprintf("process_inspector found %d processes matching %q", len(processes), name)
	}

	result := ProcessInspectorResult{
		Processes: processes,
		Count:     len(processes),
		Source:    "ps",
		Message:   "process inspection completed",
		Timestamp: now,
	}
	return result, summary, "low", nil
}

func processInspectorWindows(ctx context.Context, name string, limit int, now time.Time) (any, string, string, error) {
	_, err := exec.LookPath("tasklist")
	if err != nil {
		result := ProcessInspectorResult{
			Processes: []ProcessInfo{},
			Count:     0,
			Source:    "",
			Message:   "tasklist command not found",
			Timestamp: now,
		}
		return result, "process_inspector: tasklist not found", "low", err
	}

	args := []string{"/FO", "CSV", "/NH"}
	if name != "" {
		args = append(args, "/FI", fmt.Sprintf("IMAGENAME eq %s*", name))
	}

	output, cmdErr := runCommand(ctx, defaultCommandTimeout, "tasklist", args...)
	if cmdErr != nil {
		result := ProcessInspectorResult{
			Processes: []ProcessInfo{},
			Count:     0,
			Source:    "tasklist",
			Message:   fmt.Sprintf("tasklist failed: %v", cmdErr),
			Timestamp: now,
		}
		return result, "process_inspector: tasklist failed", "review", cmdErr
	}

	processes := make([]ProcessInfo, 0, limit)
	lines := strings.Split(output.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// tasklist CSV format: "ImageName","PID","SessionName","Session#","Mem Usage"
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		imageName := strings.Trim(parts[0], `"`)
		pidStr := strings.Trim(parts[1], `"`)
		pid := 0
		fmt.Sscanf(pidStr, "%d", &pid)

		memUsage := ""
		if len(parts) >= 5 {
			memUsage = strings.Trim(parts[4], `"`)
		}

		processes = append(processes, ProcessInfo{
			PID:   pid,
			Name:  imageName,
			State: memUsage,
			Cmd:   imageName,
		})

		if len(processes) >= limit {
			break
		}
	}

	summary := fmt.Sprintf("process_inspector found %d processes", len(processes))
	if name != "" {
		summary = fmt.Sprintf("process_inspector found %d processes matching %q", len(processes), name)
	}

	result := ProcessInspectorResult{
		Processes: processes,
		Count:     len(processes),
		Source:    "tasklist",
		Message:   "process inspection completed",
		Timestamp: now,
	}
	return result, summary, "low", nil
}
