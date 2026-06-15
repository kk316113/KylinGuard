package tools

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type ToolSemantic struct {
	OperationType     string `json:"operation_type"`
	ResourceType      string `json:"resource_type"`
	ResourcePath      string `json:"resource_path"`
	PermissionScope   string `json:"permission_scope"`
	BoundaryLevel     string `json:"boundary_level"`
	ToolSemantic      string `json:"tool_semantic"`
	RequiresPrivilege bool   `json:"requires_privilege"`
	AllowedByPolicy   bool   `json:"allowed_by_policy"`
	PolicyReason      string `json:"policy_reason"`
}

func SemanticForTool(toolName string, input map[string]any) ToolSemantic {
	switch toolName {
	case "os_info":
		return ToolSemantic{
			OperationType:     "read",
			ResourceType:      "os_info",
			ResourcePath:      "system:os",
			PermissionScope:   "public_system_info",
			BoundaryLevel:     "public",
			ToolSemantic:      "Read basic operating system information",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			PolicyReason:      "Public system information is allowed for diagnostics",
		}
	case "port_checker":
		host := stringValue(input, "host", "127.0.0.1")
		port := intValue(input, "port", 8080)
		return ToolSemantic{
			OperationType:     "inspect",
			ResourceType:      "network_port",
			ResourcePath:      "tcp:" + net.JoinHostPort(host, strconv.Itoa(port)),
			PermissionScope:   "network_port_inspect",
			BoundaryLevel:     "low",
			ToolSemantic:      "Inspect local network port status",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			PolicyReason:      "Local port inspection is allowed for diagnostics",
		}
	case "service_status":
		service := stringValue(input, "service", "system")
		return ToolSemantic{
			OperationType:     "inspect",
			ResourceType:      "system_service",
			ResourcePath:      "systemd:" + service,
			PermissionScope:   "service_status_read",
			BoundaryLevel:     "low",
			ToolSemantic:      "Inspect systemd service status",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			PolicyReason:      "Reading service status is allowed",
		}
	case "log_reader":
		return logReaderSemantic(input)
	case "safe_shell":
		return safeShellSemantic(input)
	default:
		return unknownSemantic(toolName)
	}
}

func logReaderSemantic(input map[string]any) ToolSemantic {
	path := stringValue(input, "path", "unknown-log-resource")
	sensitive := isSensitiveLogPath(path)
	semantic := ToolSemantic{
		OperationType:     "read",
		ResourceType:      "system_log",
		ResourcePath:      path,
		PermissionScope:   "system_log_read",
		BoundaryLevel:     "low",
		ToolSemantic:      "Read recent system logs for diagnosis",
		RequiresPrivilege: false,
		AllowedByPolicy:   true,
		PolicyReason:      "Log read is allowed for diagnostics",
	}
	if sensitive {
		semantic.BoundaryLevel = "sensitive_system_resource"
		semantic.RequiresPrivilege = true
		semantic.PolicyReason = "Sensitive system log read is allowed only for diagnosis and should be reviewed"
	}
	return semantic
}

func safeShellSemantic(input map[string]any) ToolSemantic {
	command := normalizeShellCommand(stringValue(input, "command", ""))
	semantic := ToolSemantic{
		OperationType:     "execute",
		ResourceType:      "shell_command",
		ResourcePath:      "command:" + command,
		PermissionScope:   "safe_command_execute",
		BoundaryLevel:     "low",
		ToolSemantic:      "Execute whitelisted safe shell command",
		RequiresPrivilege: false,
		AllowedByPolicy:   true,
		PolicyReason:      "Command is included in safe diagnostic whitelist",
	}
	if command == "" {
		semantic.PermissionScope = "unknown"
		semantic.BoundaryLevel = "dangerous"
		semantic.RequiresPrivilege = true
		semantic.AllowedByPolicy = false
		semantic.PolicyReason = "Empty shell command is not allowed"
		return semantic
	}
	if _, ok := safeCommandAllowlist[command]; ok {
		return semantic
	}
	if isDangerousShellCommand(command) || containsDangerousShellPattern(command) {
		semantic.PermissionScope = "privileged_command_execute"
		semantic.BoundaryLevel = "dangerous"
		semantic.ToolSemantic = "Blocked dangerous shell command"
		semantic.RequiresPrivilege = true
		semantic.AllowedByPolicy = false
		semantic.PolicyReason = "Command matches dangerous shell pattern and is not allowed"
		return semantic
	}
	semantic.PermissionScope = "privileged_command_execute"
	semantic.BoundaryLevel = "privileged"
	semantic.ToolSemantic = "Blocked non-whitelisted shell command"
	semantic.RequiresPrivilege = true
	semantic.AllowedByPolicy = false
	semantic.PolicyReason = "Command is not included in safe diagnostic whitelist"
	return semantic
}

func unknownSemantic(toolName string) ToolSemantic {
	return ToolSemantic{
		OperationType:     "unknown",
		ResourceType:      "unknown",
		ResourcePath:      fmt.Sprintf("tool:%s", toolName),
		PermissionScope:   "unknown",
		BoundaryLevel:     "unknown",
		ToolSemantic:      "Unknown tool semantic",
		RequiresPrivilege: false,
		AllowedByPolicy:   false,
		PolicyReason:      "No semantic mapping is registered for this tool",
	}
}

func isSensitiveLogPath(path string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	sensitive := []string{
		"/var/log/secure",
		"/var/log/auth.log",
		"/var/log/audit/audit.log",
		"/var/log/messages",
		"/var/log/syslog",
	}
	for _, candidate := range sensitive {
		if normalized == candidate || strings.HasPrefix(normalized, candidate+".") {
			return true
		}
	}
	return false
}

func isDangerousShellCommand(command string) bool {
	lower := strings.ToLower(command)
	patterns := []string{
		"rm -rf",
		"shutdown",
		"reboot",
		"mkfs",
		"dd if=",
		"chmod 777",
		"curl | sh",
		"curl -",
		"wget -",
		"systemctl stop firewalld",
		"systemctl disable firewalld",
		"iptables -f",
		"truncate -s 0 /var/log",
		"echo \"\" > /var/log",
		"echo '' > /var/log",
		"/etc/shadow",
		"sudo ",
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func normalizeShellCommand(command string) string {
	return strings.Join(strings.Fields(command), " ")
}
