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
	case "process_inspector":
		name := stringFromInput(input, "name", "")
		if name != "" && !tools.SafeProcessName(name) {
			return denyToolCall("process_inspector name must only contain letters, digits, underscore, hyphen, or dot")
		}
		limit, _ := intFromAny(input["limit"])
		if limit != 0 && (limit < 1 || limit > 100) {
			return denyToolCall("process_inspector limit must be between 1 and 100")
		}
		if hasShellInjection(input) {
			return denyToolCall("process_inspector input contains forbidden shell characters")
		}
	case "network_connection_inspector":
		state := strings.ToUpper(strings.TrimSpace(stringFromInput(input, "state", "ALL")))
		canonical := normalizeNetworkPolicyState(state)
		allowed := map[string]bool{
			"LISTEN": true, "ESTABLISHED": true,
			"TIME-WAIT": true, "CLOSE-WAIT": true, "ALL": true,
		}
		if !allowed[canonical] {
			return denyToolCall("network_connection_inspector state is not in the allowed whitelist")
		}
		limit, _ := intFromAny(input["limit"])
		if limit != 0 && (limit < 1 || limit > 500) {
			return denyToolCall("network_connection_inspector limit must be between 1 and 500")
		}
		if hasShellInjection(input) {
			return denyToolCall("network_connection_inspector input contains forbidden shell characters")
		}
	case "journalctl_reader":
		serviceName := stringFromInput(input, "service_name", "")
		if serviceName == "" {
			return denyToolCall("journalctl_reader requires a service_name")
		}
		if !tools.SafeJournalServiceName(serviceName) {
			return denyToolCall("journalctl_reader service_name contains unsafe characters; only letters, digits, underscore, hyphen, dot, and @ are allowed")
		}
		if hasShellInjection(input) {
			return denyToolCall("journalctl_reader input contains forbidden shell characters")
		}
		lines, _ := intFromAny(input["lines"])
		if lines != 0 && (lines < 1 || lines > 500) {
			return denyToolCall("journalctl_reader lines must be between 1 and 500")
		}
	case "resource_usage_checker":
		if hasShellInjection(input) {
			return denyToolCall("resource_usage_checker input contains forbidden shell characters")
		}
	case "disk_memory_checker":
		if hasShellInjection(input) {
			return denyToolCall("disk_memory_checker input contains forbidden shell characters")
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

// hasShellInjection checks for shell metacharacters in any string values of the input.
func hasShellInjection(input map[string]any) bool {
	blacklist := []string{";", "|", "&", "$", "`", ">", "<", "\n", "\r"}
	for _, value := range input {
		if text, ok := value.(string); ok {
			for _, char := range blacklist {
				if strings.Contains(text, char) {
					return true
				}
			}
		}
	}
	return false
}

// normalizeNetworkPolicyState converts user-facing state names to canonical form.
func normalizeNetworkPolicyState(state string) string {
	switch state {
	case "TIME_WAIT":
		return "TIME-WAIT"
	case "CLOSE_WAIT":
		return "CLOSE-WAIT"
	default:
		return state
	}
}
