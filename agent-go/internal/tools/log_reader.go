package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultLogLines = 100
	maxLogLines     = 500
)

var allowedLogPaths = map[string]bool{
	"/var/log/secure":          true,
	"/var/log/auth.log":        true,
	"/var/log/messages":        true,
	"/var/log/syslog":          true,
	"/var/log/audit/audit.log": true,
}

type LogReaderResult struct {
	Path           string    `json:"path"`
	AttemptedPaths []string  `json:"attempted_paths"`
	Purpose        string    `json:"purpose"`
	LinesRequested int       `json:"lines_requested"`
	LinesRead      int       `json:"lines_read"`
	Lines          []string  `json:"lines"`
	Message        string    `json:"message"`
	Errors         []string  `json:"errors"`
	Timestamp      time.Time `json:"timestamp"`
}

func LogReader(ctx context.Context, input map[string]any) (any, string, string, error) {
	_ = ctx

	paths := logPathsFromInput(input)
	lines := clampLogLines(intValue(input, "lines", defaultLogLines))
	purpose := stringValue(input, "purpose", "diagnostic_log_read")
	now := time.Now().UTC()

	result := LogReaderResult{
		AttemptedPaths: paths,
		Purpose:        purpose,
		LinesRequested: lines,
		Lines:          []string{},
		Errors:         []string{},
		Timestamp:      now,
	}

	if len(paths) == 0 {
		err := fmt.Errorf("no log paths provided")
		result.Message = err.Error()
		return result, result.Message, "review", err
	}

	for _, path := range paths {
		normalized := normalizeLogPath(path)
		if !isAllowedLogReadPath(normalized) {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: path is not allowed by log_reader policy", path))
			continue
		}

		readLines, err := readLastLines(normalized, lines)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", normalized, err.Error()))
			continue
		}

		result.Path = normalized
		result.Lines = readLines
		result.LinesRead = len(readLines)
		result.Message = fmt.Sprintf("read %d recent log lines from %s", len(readLines), normalized)
		return result, result.Message, "review", nil
	}

	err := fmt.Errorf("no allowed log path could be read: %s", strings.Join(result.Errors, "; "))
	result.Message = err.Error()
	return result, result.Message, "review", err
}

func logPathsFromInput(input map[string]any) []string {
	paths := []string{}
	if path := stringValue(input, "path", ""); path != "" {
		paths = append(paths, path)
	}

	value, ok := input["paths"]
	if !ok || value == nil {
		return dedupeStrings(paths)
	}

	switch typed := value.(type) {
	case []string:
		paths = append(paths, typed...)
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok {
				paths = append(paths, text)
			}
		}
	case string:
		paths = append(paths, typed)
	}
	return dedupeStrings(paths)
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	return result
}

func clampLogLines(lines int) int {
	if lines <= 0 {
		return defaultLogLines
	}
	if lines > maxLogLines {
		return maxLogLines
	}
	return lines
}

func normalizeLogPath(path string) string {
	return strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
}

func isAllowedLogReadPath(path string) bool {
	return allowedLogPaths[normalizeLogPath(path)]
}

func readLastLines(path string, limit int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buffer := make([]string, 0, limit)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if len(buffer) < limit {
			buffer = append(buffer, line)
			continue
		}
		copy(buffer, buffer[1:])
		buffer[len(buffer)-1] = line
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return buffer, nil
}
