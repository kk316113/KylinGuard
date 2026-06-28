package tools

import "sort"

const (
	ToolProtocol        = "mcp-like"
	ToolProtocolVersion = "stage8-v1"
)

type ToolMetadata struct {
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	Category          string         `json:"category"`
	Version           string         `json:"version"`
	InputSchema       map[string]any `json:"input_schema"`
	OutputSchema      map[string]any `json:"output_schema"`
	RiskLevel         string         `json:"risk_level"`
	PermissionScope   string         `json:"permission_scope"`
	OperationType     string         `json:"operation_type"`
	ResourceType      string         `json:"resource_type"`
	BoundaryLevel     string         `json:"boundary_level"`
	RequiresPrivilege bool           `json:"requires_privilege"`
	AllowedByPolicy   bool           `json:"allowed_by_policy"`
	Dangerous         bool           `json:"dangerous"`
	Enabled           bool           `json:"enabled"`
	DirectCallAllowed bool           `json:"direct_call_allowed"`
}

func (m ToolMetadata) ArgumentKeys() []string {
	properties, _ := m.InputSchema["properties"].(map[string]any)
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m ToolMetadata) IsReadOnly() bool {
	switch m.OperationType {
	case "read", "inspect", "analyze", "verify":
		return true
	default:
		return false
	}
}

func DefaultToolMetadata() map[string]ToolMetadata {
	return map[string]ToolMetadata{
		"os_info": {
			Name:              "os_info",
			Description:       "Collect operating system information.",
			Category:          "system",
			Version:           ToolProtocolVersion,
			InputSchema:       objectSchema(map[string]any{}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "low",
			PermissionScope:   "public_system_info",
			OperationType:     "read",
			ResourceType:      "os_info",
			BoundaryLevel:     "public",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"service_status": {
			Name:        "service_status",
			Description: "Inspect systemd service status without modifying service state.",
			Category:    "service",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"service_name": map[string]any{
					"type":        "string",
					"default":     "sshd",
					"description": "systemd service name using letters, numbers, underscore, hyphen, or dot",
				},
			}, "service_name"),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "low",
			PermissionScope:   "service_status_read",
			OperationType:     "inspect",
			ResourceType:      "system_service",
			BoundaryLevel:     "low",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"port_checker": {
			Name:        "port_checker",
			Description: "Inspect whether a local or remote TCP port is reachable.",
			Category:    "network",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"host": map[string]any{
					"type":    "string",
					"default": "127.0.0.1",
				},
				"port": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 65535,
				},
			}, "port"),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "low",
			PermissionScope:   "network_port_inspect",
			OperationType:     "inspect",
			ResourceType:      "network_port",
			BoundaryLevel:     "low",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"log_reader": {
			Name:        "log_reader",
			Description: "Read whitelisted system logs for security diagnosis.",
			Category:    "log",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"paths": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Only whitelisted log paths are allowed.",
				},
				"lines": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 500,
				},
			}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "medium",
			PermissionScope:   "system_log_read",
			OperationType:     "read",
			ResourceType:      "system_log",
			BoundaryLevel:     "sensitive_system_resource",
			RequiresPrivilege: true,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"ssh_login_analyzer": {
			Name:        "ssh_login_analyzer",
			Description: "Analyze SSH authentication logs for login anomaly diagnosis.",
			Category:    "security",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"paths": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"lines": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 500,
				},
			}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "medium",
			PermissionScope:   "ssh_auth_log_analyze",
			OperationType:     "analyze",
			ResourceType:      "ssh_auth_log",
			BoundaryLevel:     "sensitive_system_resource",
			RequiresPrivilege: true,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"safe_shell": {
			Name:        "safe_shell",
			Description: "Execute a restricted whitelist of read-only diagnostic commands.",
			Category:    "shell",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Only predefined diagnostic commands are allowed.",
				},
			}, "command"),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "medium",
			PermissionScope:   "safe_command_execute",
			OperationType:     "execute",
			ResourceType:      "safe_command",
			BoundaryLevel:     "restricted",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           false,
			DirectCallAllowed: false,
		},
		"process_inspector": {
			Name:        "process_inspector",
			Description: "Inspect process status and detect zombie processes without modifying processes.",
			Category:    "process",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Process name filter using letters, numbers, underscore, hyphen, or dot",
				},
				"limit": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 100,
				},
				"state": map[string]any{
					"type":        "string",
					"enum":        []string{"ALL", "RUNNING", "SLEEPING", "ZOMBIE", "STOPPED"},
					"default":     "ALL",
					"description": "Optional Linux process state filter",
				},
			}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "low",
			PermissionScope:   "process_inspect",
			OperationType:     "inspect",
			ResourceType:      "process",
			BoundaryLevel:     "low",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"network_connection_inspector": {
			Name:        "network_connection_inspector",
			Description: "Inspect network listening ports and connection states without modifying network configuration.",
			Category:    "network",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"state": map[string]any{
					"type":        "string",
					"default":     "ALL",
					"description": "Connection state filter: LISTEN, ESTABLISHED, TIME-WAIT, CLOSE-WAIT, ALL",
				},
				"limit": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 500,
				},
			}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "low",
			PermissionScope:   "network_connection_inspect",
			OperationType:     "inspect",
			ResourceType:      "network_connection",
			BoundaryLevel:     "low",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"journalctl_reader": {
			Name:        "journalctl_reader",
			Description: "Read recent systemd journal logs for a whitelisted service without modifying system state.",
			Category:    "log",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"service_name": map[string]any{
					"type":        "string",
					"default":     "sshd",
					"description": "systemd service name using letters, digits, underscore, hyphen, dot, or @",
				},
				"lines": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 500,
				},
			}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "medium",
			PermissionScope:   "journal_log_read",
			OperationType:     "read",
			ResourceType:      "journal_log",
			BoundaryLevel:     "sensitive_system_resource",
			RequiresPrivilege: true,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"resource_usage_checker": {
			Name:              "resource_usage_checker",
			Description:       "Read system load and memory usage from procfs for resource pressure diagnosis.",
			Category:          "resource",
			Version:           ToolProtocolVersion,
			InputSchema:       objectSchema(map[string]any{}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "low",
			PermissionScope:   "resource_usage_read",
			OperationType:     "read",
			ResourceType:      "system_resource",
			BoundaryLevel:     "low",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"disk_memory_checker": {
			Name:        "disk_memory_checker",
			Description: "Inspect disk usage and memory summary without modifying disks or mounts.",
			Category:    "resource",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"include_tmpfs": map[string]any{
					"type":        "boolean",
					"default":     false,
					"description": "Whether to include tmpfs filesystems in the output",
				},
			}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "low",
			PermissionScope:   "disk_memory_read",
			OperationType:     "read",
			ResourceType:      "disk_memory",
			BoundaryLevel:     "low",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"open_file_inspector": {
			Name:        "open_file_inspector",
			Description: "Use lsof to identify processes holding an approved operational file path or a numeric process ID; never reads file contents.",
			Category:    "process",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Approved absolute path under /var/log, /tmp, /var/tmp, or /opt/kylin-guard",
				},
				"pid": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     4194304,
					"description": "Process ID to inspect instead of path",
				},
				"limit": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 200,
				},
			}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "medium",
			PermissionScope:   "open_file_metadata_inspect",
			OperationType:     "inspect",
			ResourceType:      "open_file_metadata",
			BoundaryLevel:     "sensitive_system_resource",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"disk_io_checker": {
			Name:        "disk_io_checker",
			Description: "Sample /proc/diskstats to calculate per-device IOPS, throughput, queue depth, and utilization without modifying storage.",
			Category:    "resource",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"sample_ms": map[string]any{
					"type":        "integer",
					"minimum":     100,
					"maximum":     2000,
					"default":     250,
					"description": "Bounded sampling interval in milliseconds",
				},
			}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "low",
			PermissionScope:   "disk_io_read",
			OperationType:     "read",
			ResourceType:      "disk_io",
			BoundaryLevel:     "low",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"configuration_drift_detector": {
			Name:        "configuration_drift_detector",
			Description: "Compare installed RPM package files with the trusted RPM database baseline without reading file contents.",
			Category:    "configuration", Version: ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"packages": map[string]any{"type": "array", "minItems": 1, "maxItems": 5, "items": map[string]any{"type": "string"}},
			}, "packages"),
			OutputSchema: objectSchema(map[string]any{}), RiskLevel: "medium",
			PermissionScope: "configuration_drift_read", OperationType: "verify",
			ResourceType: "package_configuration", BoundaryLevel: "sensitive_system_resource",
			RequiresPrivilege: false, AllowedByPolicy: true, Dangerous: false, Enabled: true, DirectCallAllowed: true,
		},
		"systemd_unit_inventory": {
			Name:        "systemd_unit_inventory",
			Description: "Inventory systemd service units with read-only systemctl list-units output.",
			Category:    "service",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     500,
					"default":     100,
					"description": "Maximum number of service units to return",
				},
			}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "low",
			PermissionScope:   "systemd_unit_inventory_read",
			OperationType:     "inspect",
			ResourceType:      "system_service",
			BoundaryLevel:     "low",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"block_device_inventory": {
			Name:        "block_device_inventory",
			Description: "Inventory block devices with bounded read-only lsblk JSON output.",
			Category:    "resource",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"limit": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 500,
					"default": 100,
				},
			}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "low",
			PermissionScope:   "block_device_inventory_read",
			OperationType:     "inspect",
			ResourceType:      "block_device",
			BoundaryLevel:     "low",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"mount_inventory": {
			Name:        "mount_inventory",
			Description: "Inventory filesystem mount topology with bounded read-only findmnt JSON output.",
			Category:    "resource",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"limit": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 500,
					"default": 100,
				},
			}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "low",
			PermissionScope:   "mount_inventory_read",
			OperationType:     "inspect",
			ResourceType:      "filesystem_mount",
			BoundaryLevel:     "low",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
		"rpm_package_inventory": {
			Name:        "rpm_package_inventory",
			Description: "Inventory installed RPM packages with a bounded read-only rpm query.",
			Category:    "configuration",
			Version:     ToolProtocolVersion,
			InputSchema: objectSchema(map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Optional safe package-name substring filter",
				},
				"limit": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 500,
					"default": 100,
				},
			}),
			OutputSchema:      objectSchema(map[string]any{}),
			RiskLevel:         "low",
			PermissionScope:   "rpm_package_inventory_read",
			OperationType:     "inspect",
			ResourceType:      "package_inventory",
			BoundaryLevel:     "low",
			RequiresPrivilege: false,
			AllowedByPolicy:   true,
			Dangerous:         false,
			Enabled:           true,
			DirectCallAllowed: true,
		},
	}
}

func DefaultToolMetadataList() []ToolMetadata {
	metadata := DefaultToolMetadata()
	names := make([]string, 0, len(metadata))
	for name := range metadata {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]ToolMetadata, 0, len(names))
	for _, name := range names {
		result = append(result, metadata[name])
	}
	return result
}

func RegisteredToolCount() int {
	return len(DefaultToolMetadata())
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
