package tools

import "sort"

type CatalogSummary struct {
	Protocol        string                 `json:"protocol"`
	Version         string                 `json:"version"`
	ToolCount       int                    `json:"tool_count"`
	DirectCallCount int                    `json:"direct_call_count"`
	Categories      []ToolCategorySummary  `json:"categories"`
	SafetyModel     ToolCatalogSafetyModel `json:"safety_model"`
	ExtensionPoints []ToolCatalogExtension `json:"extension_points"`
}

type ToolCategorySummary struct {
	Category        string   `json:"category"`
	ToolCount       int      `json:"tool_count"`
	DirectCallCount int      `json:"direct_call_count"`
	SensitiveCount  int      `json:"sensitive_count"`
	Tools           []string `json:"tools"`
}

type ToolCatalogSafetyModel struct {
	DefaultMode               string   `json:"default_mode"`
	ReadOnlyDirectCallsOnly   bool     `json:"read_only_direct_calls_only"`
	UnknownToolsDefaultDenied bool     `json:"unknown_tools_default_denied"`
	RawShellExposed           bool     `json:"raw_shell_exposed"`
	AuditLayers               []string `json:"audit_layers"`
}

type ToolCatalogExtension struct {
	Name        string   `json:"name"`
	Contract    string   `json:"contract"`
	Required    []string `json:"required"`
	Description string   `json:"description"`
}

func (r *Registry) CatalogSummary() CatalogSummary {
	all := r.ListTools()
	direct := map[string]bool{}
	for _, metadata := range r.ListDirectCallTools() {
		direct[metadata.Name] = true
	}

	categoryMap := map[string]*ToolCategorySummary{}
	for _, metadata := range all {
		category := metadata.Category
		if category == "" {
			category = "uncategorized"
		}
		summary := categoryMap[category]
		if summary == nil {
			summary = &ToolCategorySummary{Category: category}
			categoryMap[category] = summary
		}
		summary.ToolCount++
		summary.Tools = append(summary.Tools, metadata.Name)
		if direct[metadata.Name] {
			summary.DirectCallCount++
		}
		if metadata.BoundaryLevel == "sensitive_system_resource" || metadata.RequiresPrivilege {
			summary.SensitiveCount++
		}
	}

	categories := make([]ToolCategorySummary, 0, len(categoryMap))
	for _, summary := range categoryMap {
		sort.Strings(summary.Tools)
		categories = append(categories, *summary)
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Category < categories[j].Category
	})

	return CatalogSummary{
		Protocol:        ToolProtocol,
		Version:         ToolProtocolVersion,
		ToolCount:       len(all),
		DirectCallCount: len(direct),
		Categories:      categories,
		SafetyModel: ToolCatalogSafetyModel{
			DefaultMode:               "least_privilege_read_only",
			ReadOnlyDirectCallsOnly:   true,
			UnknownToolsDefaultDenied: true,
			RawShellExposed:           false,
			AuditLayers:               []string{"intent_guard", "tool_policy", "exec_proxy", "tool_trace", "traceshield"},
		},
		ExtensionPoints: []ToolCatalogExtension{
			{
				Name:        "tool_metadata",
				Contract:    "register handler with ToolMetadata",
				Required:    []string{"name", "description", "category", "input_schema", "output_schema", "operation_type", "resource_type", "boundary_level", "risk_level"},
				Description: "Every tool must declare semantic and safety metadata before it can be exposed to the Agent Loop.",
			},
			{
				Name:        "tool_policy",
				Contract:    "evaluate every action before execution",
				Required:    []string{"allowed_by_policy", "dangerous", "direct_call_allowed", "read_only"},
				Description: "Unknown, dangerous, disabled, or non-read-only direct calls are denied before execution.",
			},
			{
				Name:        "audit_adapter",
				Contract:    "convert tool_trace to audit evidence",
				Required:    []string{"step_id", "tool_name", "operation_type", "resource_type", "boundary_level", "status"},
				Description: "Audit adapters receive normalized trace evidence and must not fabricate tool observations.",
			},
		},
	}
}
