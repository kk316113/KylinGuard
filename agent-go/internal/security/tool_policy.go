package security

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
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
	if unknown := unknownInputKeys(metadata, input); len(unknown) > 0 {
		return denyToolCall("tool input contains unknown arguments: " + strings.Join(unknown, ", "))
	}
	if hasShellInjection(input) {
		return denyToolCall("tool input contains forbidden shell characters")
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
		state := strings.ToUpper(strings.TrimSpace(stringFromInput(input, "state", "ALL")))
		allowedStates := map[string]bool{"ALL": true, "RUNNING": true, "SLEEPING": true, "ZOMBIE": true, "STOPPED": true}
		if !allowedStates[state] {
			return denyToolCall("process_inspector state is not in the allowed whitelist")
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
	case "journalctl_reader":
		serviceName := stringFromInput(input, "service_name", "")
		if serviceName == "" {
			return denyToolCall("journalctl_reader requires a service_name")
		}
		if !tools.SafeJournalServiceName(serviceName) {
			return denyToolCall("journalctl_reader service_name contains unsafe characters; only letters, digits, underscore, hyphen, dot, and @ are allowed")
		}
		lines, _ := intFromAny(input["lines"])
		if lines != 0 && (lines < 1 || lines > 500) {
			return denyToolCall("journalctl_reader lines must be between 1 and 500")
		}
	case "open_file_inspector":
		path := stringFromInput(input, "path", "")
		pidValue, pidProvided := input["pid"]
		pid, pidValid := intFromAny(pidValue)
		if pidProvided && !pidValid {
			return denyToolCall("open_file_inspector pid must be an integer")
		}
		if (path == "") == !pidProvided {
			return denyToolCall("open_file_inspector requires exactly one of path or pid")
		}
		if path != "" && !tools.IsAllowedOpenFilePath(path) {
			return denyToolCall("open_file_inspector path is outside the inspection allowlist")
		}
		if pidProvided && (pid < 1 || pid > 4194304) {
			return denyToolCall("open_file_inspector pid must be between 1 and 4194304")
		}
		limitValue, limitProvided := input["limit"]
		limit, limitValid := intFromAny(limitValue)
		if limitProvided && !limitValid {
			return denyToolCall("open_file_inspector limit must be an integer")
		}
		if limit != 0 && (limit < 1 || limit > 200) {
			return denyToolCall("open_file_inspector limit must be between 1 and 200")
		}
	case "disk_io_checker":
		sampleValue, sampleProvided := input["sample_ms"]
		sampleMS, sampleValid := intFromAny(sampleValue)
		if sampleProvided && !sampleValid {
			return denyToolCall("disk_io_checker sample_ms must be an integer")
		}
		if sampleMS != 0 && (sampleMS < 100 || sampleMS > 2000) {
			return denyToolCall("disk_io_checker sample_ms must be between 100 and 2000")
		}
	case "configuration_drift_detector":
		packages, valid := packageNamesForPolicy(input["packages"])
		if !valid || len(packages) < 1 || len(packages) > 5 {
			return denyToolCall("configuration_drift_detector requires 1-5 packages")
		}
		for _, name := range packages {
			if !tools.IsSafeRPMPackageName(name) {
				return denyToolCall("configuration_drift_detector package name contains unsafe characters")
			}
		}
	}

	return ToolPolicyDecision{
		Decision: "allow",
		Method:   ToolPolicyMethod,
		Message:  "tool call allowed by tool policy",
	}
}

func packageNamesForPolicy(value any) ([]string, bool) {
	names := []string{}
	switch typed := value.(type) {
	case []string:
		names = append(names, typed...)
	case []any:
		for _, item := range typed {
			if name, ok := item.(string); ok {
				names = append(names, name)
			} else {
				return nil, false
			}
		}
	default:
		return nil, false
	}
	return names, true
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
	var containsForbidden func(any) bool
	containsForbidden = func(value any) bool {
		switch typed := value.(type) {
		case string:
			for _, char := range blacklist {
				if strings.Contains(typed, char) {
					return true
				}
			}
		case []string:
			for _, item := range typed {
				if containsForbidden(item) {
					return true
				}
			}
		case []any:
			for _, item := range typed {
				if containsForbidden(item) {
					return true
				}
			}
		case map[string]any:
			for _, item := range typed {
				if containsForbidden(item) {
					return true
				}
			}
		}
		return false
	}
	for _, value := range input {
		if containsForbidden(value) {
			return true
		}
	}
	return false
}

func unknownInputKeys(metadata tools.ToolMetadata, input map[string]any) []string {
	properties, _ := metadata.InputSchema["properties"].(map[string]any)
	unknown := make([]string, 0)
	for key := range input {
		if _, ok := properties[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	return unknown
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
