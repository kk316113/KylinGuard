package tools

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/execproxy"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

// ProcessInspectorResult holds the output of process inspection.
type ProcessInspectorResult struct {
	Processes        []ProcessInfo              `json:"processes"`
	Count            int                        `json:"count"`
	ZombieCount      int                        `json:"zombie_count"`
	RiskLevel        string                     `json:"risk_level"`
	Source           string                     `json:"source"`
	Message          string                     `json:"message"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
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
func ProcessInspector(ctx context.Context, input map[string]any) (any, string, string, error) {
	name := strings.TrimSpace(stringValue(input, "name", ""))
	state := strings.ToUpper(strings.TrimSpace(stringValue(input, "state", "ALL")))
	limit := intValue(input, "limit", 20)
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	now := time.Now().UTC()

	if name != "" && !safeProcessNamePattern.MatchString(name) {
		result := ProcessInspectorResult{
			Processes:        []ProcessInfo{},
			Count:            0,
			Source:           "",
			Message:          fmt.Sprintf("invalid process name %q: only letters, digits, underscore, hyphen, and dot are allowed", name),
			Timestamp:        now,
			ExecutionContext: ecPtr(execproxy.DeniedContext("process_inspector", "invalid process name")),
		}
		return result, "process_inspector: invalid process name rejected", "low", fmt.Errorf("invalid process name %q", name)
	}

	switch runtime.GOOS {
	case "linux":
		return processInspectorLinux(ctx, name, state, limit, now)
	case "windows":
		return processInspectorWindows(ctx, name, limit, now)
	default:
		ec := execproxy.NativeExecutionContext(execproxy.ProfileLowRead, "unsupported_platform", "process_inspector unsupported on "+runtime.GOOS)
		result := ProcessInspectorResult{
			Processes:        []ProcessInfo{},
			Count:            0,
			Source:           "",
			Message:          fmt.Sprintf("process_inspector is not supported on %s", runtime.GOOS),
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return result, "process_inspector unsupported on this OS", "low", nil
	}
}

func processInspectorLinux(ctx context.Context, name string, state string, limit int, now time.Time) (any, string, string, error) {
	exec := execproxy.NewExecutor()
	result, execErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName: "process_inspector",
		Profile:  execproxy.ProfileLowRead,
		Command:  "ps",
		Args:     []string{"aux"},
		Reason:   "inspect running processes",
	})

	if execErr != nil || result.Status == "error" {
		ec := result.Context
		r := ProcessInspectorResult{
			Processes:        []ProcessInfo{},
			Count:            0,
			Source:           "ps",
			Message:          fmt.Sprintf("ps aux failed: %v", execErr),
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return r, "process_inspector: ps aux failed", "review", execErr
	}

	processes, zombieCount := parsePSAux(result.Stdout, name, state, limit)
	riskLevel := processRiskLevel(zombieCount)

	summary := fmt.Sprintf("process_inspector found %d processes; zombies=%d risk=%s", len(processes), zombieCount, riskLevel)
	if name != "" {
		summary = fmt.Sprintf("process_inspector found %d processes matching %q; zombies=%d risk=%s", len(processes), name, zombieCount, riskLevel)
	}
	if state != "" && state != "ALL" {
		summary = fmt.Sprintf("process_inspector found %d processes in state %s; zombies=%d risk=%s", len(processes), state, zombieCount, riskLevel)
	}

	r := ProcessInspectorResult{
		Processes:        processes,
		Count:            len(processes),
		ZombieCount:      zombieCount,
		RiskLevel:        riskLevel,
		Source:           "ps",
		Message:          "process inspection completed",
		Timestamp:        now,
		ExecutionContext: ecPtr(result.Context),
	}
	return r, summary, riskLevel, nil
}

func parsePSAux(output string, name string, stateFilter string, limit int) ([]ProcessInfo, int) {
	lines := strings.Split(output, "\n")
	startIdx := 0
	if len(lines) > 0 && strings.HasPrefix(strings.ToLower(lines[0]), "user") {
		startIdx = 1
	}

	processes := make([]ProcessInfo, 0, limit)
	zombieCount := 0
	for _, line := range lines[startIdx:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		procName := fields[10]

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
		user := fields[0]
		state := ""
		if len(fields) > 7 {
			state = fields[7]
		}
		if strings.HasPrefix(strings.ToUpper(state), "Z") {
			zombieCount++
		}
		if !matchesProcessState(state, stateFilter) {
			continue
		}
		cmd := procName
		if len(fields) > 10 {
			cmd = strings.Join(fields[10:], " ")
		}

		if len(processes) < limit {
			processes = append(processes, ProcessInfo{PID: pid, Name: procName, User: user, State: state, Cmd: cmd})
		}
	}

	return processes, zombieCount
}

func matchesProcessState(state string, filter string) bool {
	if filter == "" || filter == "ALL" {
		return true
	}
	state = strings.ToUpper(state)
	switch filter {
	case "RUNNING":
		return strings.HasPrefix(state, "R")
	case "SLEEPING":
		return strings.HasPrefix(state, "S") || strings.HasPrefix(state, "I")
	case "ZOMBIE":
		return strings.HasPrefix(state, "Z")
	case "STOPPED":
		return strings.HasPrefix(state, "T")
	default:
		return false
	}
}

func processRiskLevel(zombieCount int) string {
	if zombieCount >= 5 {
		return "high"
	}
	if zombieCount > 0 {
		return "medium"
	}
	return "low"
}

func processInspectorWindows(ctx context.Context, name string, limit int, now time.Time) (any, string, string, error) {
	exec := execproxy.NewExecutor()
	args := []string{"/FO", "CSV", "/NH"}
	if name != "" {
		args = append(args, "/FI", fmt.Sprintf("IMAGENAME eq %s*", name))
	}
	result, execErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName: "process_inspector",
		Profile:  execproxy.ProfileLowRead,
		Command:  "tasklist",
		Args:     args,
		Reason:   "inspect running processes on Windows",
	})

	if execErr != nil || result.Status == "error" {
		ec := result.Context
		r := ProcessInspectorResult{
			Processes:        []ProcessInfo{},
			Count:            0,
			Source:           "tasklist",
			Message:          fmt.Sprintf("tasklist failed: %v", execErr),
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return r, "process_inspector: tasklist failed", "review", execErr
	}

	processes := make([]ProcessInfo, 0, limit)
	lines := strings.Split(result.Stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
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
		processes = append(processes, ProcessInfo{PID: pid, Name: imageName, State: memUsage, Cmd: imageName})
		if len(processes) >= limit {
			break
		}
	}

	summary := fmt.Sprintf("process_inspector found %d processes", len(processes))
	if name != "" {
		summary = fmt.Sprintf("process_inspector found %d processes matching %q", len(processes), name)
	}

	r := ProcessInspectorResult{
		Processes:        processes,
		Count:            len(processes),
		ZombieCount:      0,
		RiskLevel:        "unknown",
		Source:           "tasklist",
		Message:          "process inspection completed",
		Timestamp:        now,
		ExecutionContext: ecPtr(result.Context),
	}
	return r, summary, "low", nil
}

// ecPtr converts an execproxy.ExecutionContext to *logtrace.ExecutionContext.
func ecPtr(ec execproxy.ExecutionContext) *logtrace.ExecutionContext {
	return &logtrace.ExecutionContext{
		Executor:            ec.Executor,
		Profile:             ec.Profile,
		CommandName:         ec.CommandName,
		ArgsCount:           ec.ArgsCount,
		TimeoutMs:           ec.TimeoutMs,
		MaxOutputBytes:      ec.MaxOutputBytes,
		ShellUsed:           ec.ShellUsed,
		SudoUsed:            ec.SudoUsed,
		AllowedByExecPolicy: ec.AllowedByExecPolicy,
		PolicyReason:        ec.PolicyReason,
		EffectiveUser:       ec.EffectiveUser,
		Platform:            ec.Platform,
	}
}
