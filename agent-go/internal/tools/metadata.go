package tools

import (
	"sort"
	"strings"
)

const (
	ToolProtocol        = "mcp-like"
	ToolProtocolVersion = "stage8-v1"
)

type ToolMetadata struct {
	Name              string         `json:"name"`
	DisplayName       string         `json:"display_name,omitempty"`
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
	LLMCallable       bool           `json:"llm_callable"`
	Platforms         []string       `json:"platforms,omitempty"`
	Architectures     []string       `json:"architectures,omitempty"`
	Tags              []string       `json:"tags,omitempty"`
	UseCases          []string       `json:"use_cases,omitempty"`
	SafetyNotes       []string       `json:"safety_notes,omitempty"`
	ExampleInput      map[string]any `json:"example_input,omitempty"`
	AuditEventType    string         `json:"audit_event_type,omitempty"`
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
	return withMetadataDefaults(map[string]ToolMetadata{
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
	})
}

func withMetadataDefaults(input map[string]ToolMetadata) map[string]ToolMetadata {
	result := make(map[string]ToolMetadata, len(input))
	for name, metadata := range input {
		result[name] = normalizeToolMetadata(name, metadata)
	}
	return result
}

func normalizeToolMetadata(name string, metadata ToolMetadata) ToolMetadata {
	if metadata.Name == "" {
		metadata.Name = name
	}
	if metadata.DisplayName == "" {
		metadata.DisplayName = humanizeToolName(metadata.Name)
	}
	if len(metadata.Platforms) == 0 {
		metadata.Platforms = []string{"linux", "kylin-v11"}
	}
	if len(metadata.Architectures) == 0 {
		metadata.Architectures = []string{"x86_64", "aarch64", "loongarch64"}
	}
	if len(metadata.Tags) == 0 {
		metadata.Tags = defaultToolTags(metadata)
	}
	if len(metadata.UseCases) == 0 {
		metadata.UseCases = defaultToolUseCases(metadata)
	}
	if len(metadata.SafetyNotes) == 0 {
		metadata.SafetyNotes = defaultSafetyNotes(metadata)
	}
	if metadata.ExampleInput == nil {
		metadata.ExampleInput = defaultExampleInput(metadata)
	}
	if metadata.AuditEventType == "" {
		metadata.AuditEventType = metadata.OperationType + ":" + metadata.ResourceType
	}
	metadata.LLMCallable = metadata.Enabled && metadata.DirectCallAllowed && metadata.AllowedByPolicy && metadata.IsReadOnly() && !metadata.Dangerous
	return metadata
}

func humanizeToolName(name string) string {
	words := strings.Split(strings.ReplaceAll(name, "-", "_"), "_")
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func defaultToolTags(metadata ToolMetadata) []string {
	tags := []string{"kylin", "read_only", metadata.Category, metadata.OperationType, metadata.ResourceType}
	if metadata.RequiresPrivilege {
		tags = append(tags, "privileged_read")
	}
	if metadata.RiskLevel != "" {
		tags = append(tags, "risk_"+metadata.RiskLevel)
	}
	return compactUnique(tags)
}

func defaultToolUseCases(metadata ToolMetadata) []string {
	switch metadata.Category {
	case "security":
		return []string{"ssh_login_diagnosis", "authentication_anomaly_review", "security_event_triage"}
	case "service":
		return []string{"service_recovery", "systemd_inventory", "availability_diagnosis"}
	case "network":
		return []string{"port_reachability_check", "connection_state_review", "service_access_diagnosis"}
	case "log":
		return []string{"journal_review", "system_log_triage", "incident_evidence_collection"}
	case "resource":
		return []string{"system_health_check", "performance_bottleneck_triage", "capacity_review"}
	case "configuration":
		return []string{"configuration_drift_review", "rpm_inventory", "baseline_integrity_check"}
	case "process":
		return []string{"process_health_check", "open_file_review", "resource_owner_triage"}
	default:
		return []string{"ops_diagnosis", "read_only_inspection"}
	}
}

func defaultSafetyNotes(metadata ToolMetadata) []string {
	notes := []string{
		"Read-only by default; no system mutation is performed.",
		"Every call is mediated by Tool Policy and Exec Proxy.",
		"Tool output is converted into tool_trace and audited before being shown as evidence.",
	}
	if metadata.RequiresPrivilege {
		notes = append(notes, "May require elevated read permission on real Kylin systems.")
	}
	if metadata.Name == "safe_shell" {
		notes = append(notes, "Not exposed for LLM/direct calls; only a conservative internal whitelist may execute.")
	}
	return notes
}

func defaultExampleInput(metadata ToolMetadata) map[string]any {
	switch metadata.Name {
	case "service_status":
		return map[string]any{"service_name": "sshd"}
	case "journalctl_reader":
		return map[string]any{"service_name": "sshd", "lines": 100}
	case "log_reader", "ssh_login_analyzer":
		return map[string]any{"lines": 100}
	case "port_checker":
		return map[string]any{"host": "127.0.0.1", "port": 22}
	case "process_inspector":
		return map[string]any{"limit": 20, "state": "ALL"}
	case "network_connection_inspector":
		return map[string]any{"state": "LISTEN", "limit": 100}
	case "disk_memory_checker":
		return map[string]any{"include_tmpfs": false}
	case "disk_io_checker":
		return map[string]any{"sample_ms": 500}
	case "configuration_drift_detector":
		return map[string]any{"packages": []string{"openssh-server"}}
	case "rpm_package_inventory":
		return map[string]any{"query": "openssh", "limit": 50}
	case "systemd_unit_inventory", "block_device_inventory", "mount_inventory":
		return map[string]any{"limit": 100}
	case "open_file_inspector":
		return map[string]any{"path": "/var/log/secure", "limit": 20}
	case "safe_shell":
		return map[string]any{"command": "uname -a"}
	default:
		return map[string]any{}
	}
}

func compactUnique(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
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
