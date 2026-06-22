package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/execproxy"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

type OpenFileInspectorResult struct {
	OpenFiles        []OpenFileInfo             `json:"open_files"`
	Count            int                        `json:"count"`
	Query            string                     `json:"query"`
	Source           string                     `json:"source"`
	Message          string                     `json:"message"`
	Timestamp        time.Time                  `json:"timestamp"`
	ExecutionContext *logtrace.ExecutionContext `json:"-"`
}

type OpenFileInfo struct {
	PID     int    `json:"pid"`
	Command string `json:"command"`
	UserID  string `json:"user_id,omitempty"`
	FD      string `json:"fd,omitempty"`
	Type    string `json:"type,omitempty"`
	Name    string `json:"name,omitempty"`
}

// IsAllowedOpenFilePath restricts lsof path inspection to operational data
// areas. The tool reports which process owns a file; it never reads contents.
func IsAllowedOpenFilePath(path string) bool {
	normalized := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if !strings.HasPrefix(normalized, "/") {
		return false
	}
	allowedPrefixes := []string{
		"/var/log/",
		"/tmp/",
		"/var/tmp/",
		"/opt/kylin-guard/",
	}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

func OpenFileInspector(ctx context.Context, input map[string]any) (any, string, string, error) {
	path := strings.TrimSpace(stringValue(input, "path", ""))
	pid := intValue(input, "pid", 0)
	limit := intValue(input, "limit", 50)
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	now := time.Now().UTC()
	query := path
	if pid > 0 {
		query = fmt.Sprintf("pid:%d", pid)
	}

	if runtime.GOOS != "linux" {
		ec := execproxy.NativeExecutionContext(execproxy.ProfileSensitiveRead, "unsupported_platform", "lsof inspection is only supported on Linux")
		result := OpenFileInspectorResult{
			OpenFiles:        []OpenFileInfo{},
			Query:            query,
			Source:           "",
			Message:          "open_file_inspector is only supported on Linux",
			Timestamp:        now,
			ExecutionContext: ecPtr(ec),
		}
		return result, "open_file_inspector unsupported on non-Linux host", "low", nil
	}

	args := []string{"-nP", "-F", "pcuftn"}
	if path != "" {
		args = append(args, "--", filepath.Clean(path))
	} else {
		args = append(args, "-p", strconv.Itoa(pid))
	}

	executor := execproxy.NewExecutor()
	execResult, execErr := executor.Execute(ctx, execproxy.CommandSpec{
		ToolName:         "open_file_inspector",
		Profile:          execproxy.ProfileSensitiveRead,
		Command:          "lsof",
		Args:             args,
		AllowNonZeroExit: true,
		SensitiveOutput:  true,
		Reason:           "inspect open-file ownership without reading file contents",
	})
	if execErr != nil && !(execResult.ExitCode == 1 && strings.TrimSpace(execResult.Stdout) == "") {
		result := OpenFileInspectorResult{
			OpenFiles:        []OpenFileInfo{},
			Query:            query,
			Source:           "lsof",
			Message:          fmt.Sprintf("lsof inspection failed: %v", execErr),
			Timestamp:        now,
			ExecutionContext: ecPtr(execResult.Context),
		}
		return result, "open_file_inspector: lsof failed", "review", execErr
	}

	files := parseLsofFieldOutput(execResult.Stdout, limit)
	result := OpenFileInspectorResult{
		OpenFiles:        files,
		Count:            len(files),
		Query:            query,
		Source:           "lsof",
		Message:          "open-file ownership inspection completed",
		Timestamp:        now,
		ExecutionContext: ecPtr(execResult.Context),
	}
	return result, fmt.Sprintf("open_file_inspector found %d open-file records for %s", len(files), query), "medium", nil
}

func parseLsofFieldOutput(output string, limit int) []OpenFileInfo {
	files := make([]OpenFileInfo, 0)
	currentProcess := OpenFileInfo{}
	currentFile := OpenFileInfo{}
	flushFile := func() {
		if currentFile.FD == "" && currentFile.Name == "" {
			return
		}
		currentFile.PID = currentProcess.PID
		currentFile.Command = currentProcess.Command
		currentFile.UserID = currentProcess.UserID
		files = append(files, currentFile)
		currentFile = OpenFileInfo{}
	}

	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		field, value := line[0], line[1:]
		switch field {
		case 'p':
			flushFile()
			currentProcess = OpenFileInfo{}
			currentProcess.PID, _ = strconv.Atoi(value)
		case 'c':
			currentProcess.Command = value
		case 'u':
			currentProcess.UserID = value
		case 'f':
			flushFile()
			currentFile.FD = value
		case 't':
			currentFile.Type = value
		case 'n':
			currentFile.Name = value
		}
		if len(files) >= limit {
			return files[:limit]
		}
	}
	flushFile()
	if len(files) > limit {
		files = files[:limit]
	}
	return files
}
