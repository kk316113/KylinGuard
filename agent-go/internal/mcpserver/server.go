package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

const (
	ServerName    = "kylin-guard-mcp"
	ServerVersion = "0.1.0"
)

// Dependencies are the existing KylinGuard safety and execution components
// used by the MCP transport. MCP calls deliberately share the same registry,
// Tool Policy, trace store, and auditor as the Agent Loop.
type Dependencies struct {
	Registry   *tools.Registry
	Policy     security.ToolPolicy
	TraceStore *logtrace.Store
	Auditor    auditclient.Client
}

// NewServer exposes policy-approved KylinGuard tools through the official MCP
// Go SDK. Tools that are disabled, dangerous, or not allowed for direct calls
// are not advertised to MCP clients.
func NewServer(deps Dependencies) *mcp.Server {
	if deps.Registry == nil {
		deps.Registry = tools.NewDefaultRegistry()
	}
	if deps.TraceStore == nil {
		deps.TraceStore = logtrace.NewStore()
	}
	if deps.Auditor == nil {
		deps.Auditor = auditclient.NewMockClient()
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    ServerName,
		Version: ServerVersion,
	}, nil)

	for _, metadata := range deps.Registry.ListTools() {
		if !deps.Registry.IsToolEnabledForDirectCall(metadata.Name) {
			continue
		}

		meta := metadata
		readOnly := meta.OperationType != "write" && meta.OperationType != "execute"
		destructive := meta.Dangerous
		openWorld := meta.ResourceType == "network_port"

		mcp.AddTool(server, &mcp.Tool{
			Name:         meta.Name,
			Description:  meta.Description,
			InputSchema:  meta.InputSchema,
			OutputSchema: meta.OutputSchema,
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint:    readOnly,
				DestructiveHint: &destructive,
				IdempotentHint:  readOnly,
				OpenWorldHint:   &openWorld,
			},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
			return callTool(ctx, deps, meta, input)
		})
	}

	return server
}

// NewHTTPHandler serves MCP Streamable HTTP at the route selected by the
// caller. Stateless mode avoids retaining abandoned competition/demo sessions.
func NewHTTPHandler(deps Dependencies) http.Handler {
	server := NewServer(deps)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless:      true,
		SessionTimeout: 5 * time.Minute,
	})
}

func callTool(ctx context.Context, deps Dependencies, metadata tools.ToolMetadata, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	if input == nil {
		input = map[string]any{}
	}

	decision := deps.Policy.Evaluate(metadata.Name, metadata, true, input)
	if decision.Decision == "deny" {
		trace := deniedTrace(metadata, input, decision)
		deps.TraceStore.Add(trace)
		audit, auditErr := deps.Auditor.AuditTrace(ctx, mcpTask(metadata.Name), []logtrace.ToolTrace{trace})
		if auditErr != nil {
			audit.Message = appendMessage(audit.Message, auditErr.Error())
		}
		audit.Decision = "deny"
		audit.RiskScore = 1.0
		audit.Method = security.ToolPolicyMethod
		audit.Message = decisionMessage(decision)
		audit.Violations = append(audit.Violations, auditclient.Violation{
			Type:     "tool_policy",
			Severity: "high",
			Message:  decisionMessage(decision),
			StepID:   trace.StepID,
		})
		audit.EvidenceChain = append(audit.EvidenceChain, auditclient.EvidenceItem{
			StepID:   trace.StepID,
			ToolName: metadata.Name,
			Resource: trace.ResourcePath,
			Reason:   decision.Reason,
		})
		return &mcp.CallToolResult{IsError: true}, map[string]any{
			"tool_name":    metadata.Name,
			"status":       "denied",
			"message":      decisionMessage(decision),
			"trace":        trace,
			"audit_result": audit,
		}, nil
	}

	result, toolErr := deps.Registry.CallTool(ctx, metadata.Name, input)
	deps.TraceStore.Add(result.Trace)
	audit, auditErr := deps.Auditor.AuditTrace(ctx, mcpTask(metadata.Name), []logtrace.ToolTrace{result.Trace})

	status := result.Trace.Status
	message := "tool call passed Tool Policy and was audited"
	if toolErr != nil {
		status = "error"
		message = toolErr.Error()
	}
	if auditErr != nil {
		status = "error"
		message = appendMessage(message, "audit failed: "+auditErr.Error())
	}

	output := map[string]any{
		"tool_name":    metadata.Name,
		"status":       status,
		"trace":        result.Trace,
		"audit_result": audit,
	}
	if result.Output != nil {
		output["output"] = result.Output
	}
	if strings.TrimSpace(message) != "" {
		output["message"] = message
	}

	return &mcp.CallToolResult{IsError: toolErr != nil || auditErr != nil}, output, nil
}

func deniedTrace(metadata tools.ToolMetadata, input map[string]any, decision security.ToolPolicyDecision) logtrace.ToolTrace {
	now := time.Now().UTC()
	return logtrace.ToolTrace{
		StepID:            logtrace.NextStepID(),
		ToolName:          metadata.Name,
		Input:             input,
		OutputSummary:     decisionMessage(decision),
		Status:            "denied",
		StartedAt:         now,
		FinishedAt:        now,
		RiskHint:          "high",
		OperationType:     metadata.OperationType,
		ResourceType:      metadata.ResourceType,
		PermissionScope:   metadata.PermissionScope,
		BoundaryLevel:     metadata.BoundaryLevel,
		ToolSemantic:      fmt.Sprintf("MCP tool call denied before execution: %s", metadata.Name),
		RequiresPrivilege: metadata.RequiresPrivilege,
		AllowedByPolicy:   false,
		PolicyReason:      decision.Reason,
	}
}

func mcpTask(toolName string) string {
	return "MCP structured tool call: " + toolName
}

func decisionMessage(decision security.ToolPolicyDecision) string {
	if strings.TrimSpace(decision.Reason) == "" {
		return decision.Message
	}
	return decision.Message + ": " + decision.Reason
}

func appendMessage(base, detail string) string {
	base = strings.TrimSpace(base)
	detail = strings.TrimSpace(detail)
	if base == "" {
		return detail
	}
	if detail == "" {
		return base
	}
	return base + "; " + detail
}
