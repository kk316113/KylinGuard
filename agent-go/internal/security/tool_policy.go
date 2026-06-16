package security

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"kylin-guard-agent/agent-go/internal/tools"
)

const ToolPolicyMethod = "tool_policy"

type ToolPolicy struct{}

type ToolPolicyDecision struct {
	Decision string `json:"decision"`
	Method   string `json:"method"`
	Message  string `json:"message"`
	Reason   string `json:"reason,omitempty"`
}

func NewToolPolicy() ToolPolicy {
	return ToolPolicy{}
}

func (ToolPolicy) Evaluate(name string, metadata tools.ToolMetadata, exists bool, input map[string]any) ToolPolicyDecision {
	if strings.TrimSpace(name) == "" {
		return denyToolCall("tool name is required")
	}
	if !exists {
		return denyToolCall("unknown tool")
	}
	if !metadata.Enabled {
		return denyToolCall("tool is disabled for direct calls")
	}
	if !metadata.DirectCallAllowed {
		return denyToolCall("tool direct call is disabled")
	}
	if metadata.Dangerous {
		return denyToolCall("dangerous tool is not allowed")
	}
	if !metadata.AllowedByPolicy {
		return denyToolCall("tool metadata is not allowed by policy")
	}
	if input == nil {
		input = map[string]any{}
	}

	switch name {
	case "safe_shell":
		return denyToolCall("safe_shell direct call is disabled")
	case "port_checker":
		port, ok := intFromAny(input["port"])
		if !ok || port < 1 || port > 65535 {
			return denyToolCall("port_checker port must be between 1 and 65535")
		}
	case "log_reader":
		paths := pathsFromInput(input)
		if len(paths) == 0 {
			return denyToolCall("log_reader requires at least one whitelisted path")
		}
		for _, path := range paths {
			if !tools.IsAllowedLogReadPath(path) {
				return denyToolCall(fmt.Sprintf("log path is not whitelisted: %s", tools.NormalizeLogPath(path)))
			}
		}
	case "ssh_login_analyzer":
		paths := pathsFromInput(input)
		for _, path := range paths {
			if isJournalctlSSHD(path) {
				continue
			}
			if !tools.IsAllowedLogReadPath(path) {
				return denyToolCall(fmt.Sprintf("ssh auth log path is not whitelisted: %s", tools.NormalizeLogPath(path)))
			}
		}
	case "service_status":
		service := stringFromInput(input, "service_name", "")
		if service == "" {
			service = stringFromInput(input, "service", "sshd")
		}
		if !safeServiceName(service) {
			return denyToolCall("service_name contains unsafe characters")
		}
	}

	return ToolPolicyDecision{
		Decision: "allow",
		Method:   ToolPolicyMethod,
		Message:  "tool call allowed by tool policy",
	}
}

func denyToolCall(reason string) ToolPolicyDecision {
	return ToolPolicyDecision{
		Decision: "deny",
		Method:   ToolPolicyMethod,
		Message:  "tool call denied by tool policy",
		Reason:   reason,
	}
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), typed == float64(int(typed))
	case json.Number:
		parsed, err := strconv.Atoi(string(typed))
		return parsed, err == nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		return 0, false
	}
}

func stringFromInput(input map[string]any, key string, fallback string) string {
	value, ok := input[key]
	if !ok || value == nil {
		return fallback
	}
	if text, ok := value.(string); ok {
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func pathsFromInput(input map[string]any) []string {
	paths := []string{}
	if path := stringFromInput(input, "path", ""); path != "" {
		paths = append(paths, path)
	}
	value, ok := input["paths"]
	if !ok || value == nil {
		return paths
	}

	switch typed := value.(type) {
	case []string:
		paths = append(paths, typed...)
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				paths = append(paths, text)
			}
		}
	case string:
		if strings.TrimSpace(typed) != "" {
			paths = append(paths, typed)
		}
	}
	return paths
}

func isJournalctlSSHD(value string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(value), " "))
	return normalized == "journalctl:sshd" || normalized == "journalctl -u sshd" || normalized == "journalctl sshd"
}

var serviceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func safeServiceName(service string) bool {
	service = strings.TrimSpace(service)
	return service != "" && serviceNamePattern.MatchString(service)
}
