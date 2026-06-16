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
