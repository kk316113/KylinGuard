package tools

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"kylin-guard-agent/agent-go/internal/execproxy"
)

const defaultAuthLogLines = 200

var defaultAuthLogPaths = []string{
	"/var/log/secure",
	"/var/log/auth.log",
}

type LogCollectionResult struct {
	SourceType     string   `json:"source_type"`
	SourcePath     string   `json:"source_path"`
	Lines          []string `json:"lines"`
	LinesRequested int      `json:"lines_requested"`
	Status         string   `json:"status"`
	Message        string   `json:"message"`
	AttemptedPaths []string `json:"attempted_paths,omitempty"`
	Errors         []string `json:"errors,omitempty"`
}

func collectSSHAuthLogs(ctx context.Context, input map[string]any) (LogCollectionResult, error) {
	lines := clampLogLines(intValue(input, "lines", defaultAuthLogLines))
	paths := logPathsFromInput(input)
	if len(paths) == 0 {
		paths = append([]string{}, defaultAuthLogPaths...)
	}

	result := LogCollectionResult{
		SourceType:     "none",
		Lines:          []string{},
		LinesRequested: lines,
		Status:         "error",
		AttemptedPaths: paths,
		Errors:         []string{},
	}

	for _, path := range paths {
		normalized := normalizeLogPath(path)
		if !isAllowedLogReadPath(normalized) {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: path is not allowed by auth log collector policy", path))
			continue
		}

		readLines, err := readLastLines(normalized, lines)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", normalized, err.Error()))
			continue
		}

		result.SourceType = "file"
		result.SourcePath = normalized
		result.Lines = readLines
		result.Status = "ok"
		result.Message = fmt.Sprintf("collected %d auth log lines from %s", len(readLines), normalized)
		return result, nil
	}

	if runtime.GOOS != "linux" {
		return finishLogCollectionError(result, "file logs unavailable and journalctl is only checked on Linux targets")
	}

	exec := execproxy.NewExecutor()
	execResult, execErr := exec.Execute(ctx, execproxy.CommandSpec{
		ToolName:         "ssh_login_analyzer",
		Profile:          execproxy.ProfileSensitiveRead,
		Command:          "journalctl",
		Args:             []string{"-u", "sshd", "-n", strconv.Itoa(lines), "--no-pager"},
		SensitiveOutput:  true,
		AllowNonZeroExit: true,
		Reason:           "collect SSH auth logs for anomaly analysis",
	})
	if execErr != nil {
		result.Errors = append(result.Errors, "journalctl -u sshd: "+execErr.Error())
		return finishLogCollectionError(result, "all auth log sources unavailable")
	}

	collected := splitLogLines(execResult.Stdout)
	result.SourceType = "journalctl"
	result.SourcePath = "journalctl:sshd"
	result.Lines = collected
	result.Status = "ok"
	result.Message = fmt.Sprintf("collected %d auth log lines from journalctl -u sshd", len(collected))
	return result, nil
}

func finishLogCollectionError(result LogCollectionResult, message string) (LogCollectionResult, error) {
	if len(result.Errors) > 0 {
		message = message + ": " + strings.Join(result.Errors, "; ")
	}
	result.SourceType = "none"
	result.SourcePath = ""
	result.Status = "error"
	result.Message = message
	return result, fmt.Errorf("%s", message)
}

func splitLogLines(text string) []string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}
