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
		service := serviceNameFromInput(input)
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
	case "ssh_login_analyzer":
		return sshLoginAnalyzerSemantic()
	case "safe_shell":
		return safeShellSemantic(input)
	case "process_inspector":
		return processInspectorSemantic(input)
	case "network_connection_inspector":
		return networkConnectionInspectorSemantic(input)
	case "journalctl_reader":
		return journalctlReaderSemantic(input)
	case "resource_usage_checker":
		return resourceUsageCheckerSemantic()
	case "disk_memory_checker":
		return diskMemoryCheckerSemantic()
	case "open_file_inspector":
		return openFileInspectorSemantic(input)
	case "disk_io_checker":
		return diskIOCheckerSemantic()
	default:
		return unknownSemantic(toolName)
	}
}

func sshLoginAnalyzerSemantic() ToolSemantic {
	return ToolSemantic{
		OperationType:     "analyze",
		ResourceType:      "ssh_auth_log",
		ResourcePath:      "ssh_auth:none",
		PermissionScope:   "ssh_auth_log_analyze",
		BoundaryLevel:     "sensitive_system_resource",
		ToolSemantic:      "ssh_login_anomaly_analysis",
		RequiresPrivilege: true,
		AllowedByPolicy:   true,
		PolicyReason:      "SSH authentication log analysis is allowed for the user-requested SSH anomaly diagnosis task",
	}
}

func logReaderSemantic(input map[string]any) ToolSemantic {
	paths := logPathsFromInput(input)
	path := "unknown-log-resource"
	if len(paths) > 0 {
		path = strings.Join(paths, ",")
	}
	allowed := len(paths) > 0
	requiresPrivilege := false
	for _, candidate := range paths {
		normalized := normalizeLogPath(candidate)
		if !isAllowedLogReadPath(normalized) {
			allowed = false
		}
		if isPrivilegedLogPath(normalized) {
			requiresPrivilege = true
		}
	}
	semantic := ToolSemantic{
		OperationType:     "read",
		ResourceType:      "system_log",
		ResourcePath:      path,
		PermissionScope:   "system_log_read",
		BoundaryLevel:     "sensitive_system_resource",
		ToolSemantic:      "Read recent system logs for diagnosis",
		RequiresPrivilege: requiresPrivilege,
		AllowedByPolicy:   allowed,
		PolicyReason:      "Whitelisted system log read is allowed for diagnostics and should be reviewed",
	}
	if !allowed {
		semantic.PermissionScope = "blocked_file_read"
		semantic.BoundaryLevel = "dangerous"
		semantic.RequiresPrivilege = true
		semantic.AllowedByPolicy = false
		semantic.PolicyReason = "Requested log path is outside the log_reader whitelist"
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

func isPrivilegedLogPath(path string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	sensitive := []string{
		"/var/log/secure",
		"/var/log/auth.log",
		"/var/log/audit/audit.log",
	}
	for _, candidate := range sensitive {
		if normalized == candidate {
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

func processInspectorSemantic(input map[string]any) ToolSemantic {
	name := strings.TrimSpace(stringValue(input, "name", ""))
	resourcePath := "process:all"
	if name != "" {
		resourcePath = "process:" + name
	}
	if state := strings.ToUpper(strings.TrimSpace(stringValue(input, "state", "ALL"))); state != "" && state != "ALL" {
		resourcePath += ":state=" + state
	}
	return ToolSemantic{
		OperationType:     "inspect",
		ResourceType:      "process",
		ResourcePath:      resourcePath,
		PermissionScope:   "process_inspect",
		BoundaryLevel:     "low",
		ToolSemantic:      "process_status_inspection",
		RequiresPrivilege: false,
		AllowedByPolicy:   true,
		PolicyReason:      "Process status inspection is allowed for diagnostics",
	}
}

func openFileInspectorSemantic(input map[string]any) ToolSemantic {
	resourcePath := strings.TrimSpace(stringValue(input, "path", ""))
	if pid := intValue(input, "pid", 0); pid > 0 {
		resourcePath = fmt.Sprintf("process:%d:open-files", pid)
	}
	return ToolSemantic{
		OperationType:     "inspect",
		ResourceType:      "open_file_metadata",
		ResourcePath:      resourcePath,
		PermissionScope:   "open_file_metadata_inspect",
		BoundaryLevel:     "sensitive_system_resource",
		ToolSemantic:      "open_file_ownership_inspection",
		RequiresPrivilege: false,
		AllowedByPolicy:   true,
		PolicyReason:      "Bounded lsof metadata inspection is allowed for diagnosis; file contents are not read",
	}
}

func diskIOCheckerSemantic() ToolSemantic {
	return ToolSemantic{
		OperationType:     "read",
		ResourceType:      "disk_io",
		ResourcePath:      "procfs:diskstats",
		PermissionScope:   "disk_io_read",
		BoundaryLevel:     "low",
		ToolSemantic:      "disk_io_pressure_sampling",
		RequiresPrivilege: false,
		AllowedByPolicy:   true,
		PolicyReason:      "Read-only disk statistics sampling is allowed for performance diagnosis",
	}
}

func networkConnectionInspectorSemantic(input map[string]any) ToolSemantic {
	state := strings.ToUpper(strings.TrimSpace(stringValue(input, "state", "ALL")))
	resourcePath := "network:connections:" + state
	return ToolSemantic{
		OperationType:     "inspect",
		ResourceType:      "network_connection",
		ResourcePath:      resourcePath,
		PermissionScope:   "network_connection_inspect",
		BoundaryLevel:     "low",
		ToolSemantic:      "network_connection_inspection",
		RequiresPrivilege: false,
		AllowedByPolicy:   true,
		PolicyReason:      "Network connection inspection is allowed for diagnostics",
	}
}

func journalctlReaderSemantic(input map[string]any) ToolSemantic {
	serviceName := strings.TrimSpace(stringValue(input, "service_name", "sshd"))
	resourcePath := "journalctl:" + serviceName
	return ToolSemantic{
		OperationType:     "read",
		ResourceType:      "journal_log",
		ResourcePath:      resourcePath,
		PermissionScope:   "journal_log_read",
		BoundaryLevel:     "sensitive_system_resource",
		ToolSemantic:      "journal_log_read",
		RequiresPrivilege: true,
		AllowedByPolicy:   true,
		PolicyReason:      "Journal log read is allowed for security diagnosis",
	}
}

func resourceUsageCheckerSemantic() ToolSemantic {
	return ToolSemantic{
		OperationType:     "read",
		ResourceType:      "system_resource",
		ResourcePath:      "procfs:loadavg,meminfo",
		PermissionScope:   "resource_usage_read",
		BoundaryLevel:     "low",
		ToolSemantic:      "resource_usage_read",
		RequiresPrivilege: false,
		AllowedByPolicy:   true,
		PolicyReason:      "System resource usage read is allowed for diagnostics",
	}
}

func diskMemoryCheckerSemantic() ToolSemantic {
	return ToolSemantic{
		OperationType:     "read",
		ResourceType:      "disk_memory",
		ResourcePath:      "system:disk_memory",
		PermissionScope:   "disk_memory_read",
		BoundaryLevel:     "low",
		ToolSemantic:      "disk_memory_read",
		RequiresPrivilege: false,
		AllowedByPolicy:   true,
		PolicyReason:      "Disk and memory read is allowed for diagnostics",
	}
}

func normalizeShellCommand(command string) string {
	return strings.Join(strings.Fields(command), " ")
}
