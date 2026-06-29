package auditclient

import (
	"fmt"
	"strings"
)

type SemanticGraphStep struct {
	StepID            string
	StepIndex         int
	ToolName          string
	Decision          string
	RiskScore         float64
	RiskLevel         string
	Method            string
	Status            string
	PolicyDecision    string
	OperationType     string
	ResourceType      string
	ResourcePath      string
	BoundaryLevel     string
	AllowedByPolicy   bool
	PolicyReason      string
	ViolationsCount   int
	RequiresPrivilege bool
	RiskHint          string
}

func RiskGraphFromSemanticSteps(steps []SemanticGraphStep) *RiskGraph {
	if len(steps) == 0 {
		return nil
	}

	builder := semanticGraphBuilder{
		nodes:             make([]map[string]any, 0, len(steps)*4),
		edges:             make([]map[string]any, 0, len(steps)*6),
		seenNodes:         make(map[string]bool),
		seenEdges:         make(map[string]bool),
		riskHotspots:      make([]map[string]any, 0),
		boundaryCrossings: make([]map[string]any, 0),
		decisionPath:      make([]map[string]any, 0, len(steps)),
	}

	var previousStepID string
	for index, raw := range steps {
		step := normalizeSemanticStep(raw, index)
		stepID := step.StepID
		operationID := semanticNodeID("operation", step.OperationType)
		resourceID := semanticResourceID(step)
		boundaryID := semanticNodeID("boundary", step.BoundaryLevel)
		policyID := semanticNodeID("policy", policyNodeValue(step))
		decisionID := semanticNodeID("decision", decisionNodeValue(step))

		builder.addNode(stepID, map[string]any{
			"id":                 stepID,
			"step_id":            stepID,
			"step_index":         step.StepIndex,
			"type":               "tool_call",
			"label":              step.ToolName,
			"tool_name":          step.ToolName,
			"decision":           step.Decision,
			"risk_score":         step.RiskScore,
			"risk_level":         semanticRiskLevel(step),
			"violations_count":   step.ViolationsCount,
			"method":             step.Method,
			"status":             step.Status,
			"policy_decision":    step.PolicyDecision,
			"operation_type":     step.OperationType,
			"resource_type":      step.ResourceType,
			"resource_path":      step.ResourcePath,
			"boundary_level":     step.BoundaryLevel,
			"allowed_by_policy":  step.AllowedByPolicy,
			"policy_reason":      step.PolicyReason,
			"requires_privilege": step.RequiresPrivilege,
			"risk_hint":          step.RiskHint,
			"semantic_role":      "tool_event",
			"layer":              1,
		})
		builder.addNode(operationID, map[string]any{
			"id":             operationID,
			"type":           "operation",
			"label":          step.OperationType,
			"operation_type": step.OperationType,
			"risk_level":     semanticOperationRisk(step.OperationType),
			"semantic_role":  "operation",
			"layer":          2,
		})
		builder.addNode(resourceID, map[string]any{
			"id":            resourceID,
			"type":          "resource",
			"label":         semanticResourceLabel(step),
			"resource_type": step.ResourceType,
			"resource_path": step.ResourcePath,
			"risk_level":    semanticResourceRisk(step),
			"semantic_role": "resource",
			"layer":         3,
		})
		builder.addNode(boundaryID, map[string]any{
			"id":             boundaryID,
			"type":           "boundary",
			"label":          step.BoundaryLevel,
			"boundary_level": step.BoundaryLevel,
			"risk_level":     semanticBoundaryRisk(step.BoundaryLevel),
			"semantic_role":  "boundary",
			"layer":          4,
		})
		builder.addNode(policyID, map[string]any{
			"id":                policyID,
			"type":              "policy",
			"label":             policyNodeValue(step),
			"policy_decision":   step.PolicyDecision,
			"allowed_by_policy": step.AllowedByPolicy,
			"policy_reason":     step.PolicyReason,
			"risk_level":        semanticPolicyRisk(step),
			"semantic_role":     "policy_guard",
			"layer":             0,
		})
		builder.addNode(decisionID, map[string]any{
			"id":            decisionID,
			"type":          "decision",
			"label":         decisionNodeValue(step),
			"decision":      step.Decision,
			"risk_level":    semanticRiskLevel(step),
			"semantic_role": "audit_decision",
			"layer":         5,
		})

		builder.addEdge(policyID, stepID, "governs", "policy check", semanticPolicyRisk(step))
		builder.addEdge(stepID, operationID, "performs", "performs", semanticOperationRisk(step.OperationType))
		builder.addEdge(operationID, resourceID, "targets", "targets resource", semanticResourceRisk(step))
		builder.addEdge(resourceID, boundaryID, "crosses_boundary", "boundary", semanticBoundaryRisk(step.BoundaryLevel))
		builder.addEdge(stepID, decisionID, "audited_as", "audit decision", semanticRiskLevel(step))
		if previousStepID != "" {
			builder.addEdge(previousStepID, stepID, "next_action", "next action", "low")
		}
		previousStepID = stepID

		if semanticRiskLevel(step) == "high" || semanticRiskLevel(step) == "medium" {
			builder.riskHotspots = append(builder.riskHotspots, map[string]any{
				"step_id":    stepID,
				"tool_name":  step.ToolName,
				"risk_level": semanticRiskLevel(step),
				"summary":    semanticHotspotSummary(step),
			})
		}
		if isBoundaryCrossing(step.BoundaryLevel) {
			builder.boundaryCrossings = append(builder.boundaryCrossings, map[string]any{
				"step_id":        stepID,
				"resource_type":  step.ResourceType,
				"resource_path":  step.ResourcePath,
				"boundary_level": step.BoundaryLevel,
				"decision":       step.Decision,
			})
		}
		builder.decisionPath = append(builder.decisionPath, map[string]any{
			"step_id":         stepID,
			"step_index":      step.StepIndex,
			"tool_name":       step.ToolName,
			"policy_decision": step.PolicyDecision,
			"audit_decision":  step.Decision,
			"risk_level":      semanticRiskLevel(step),
		})
	}

	return &RiskGraph{
		Nodes:             builder.nodes,
		Edges:             builder.edges,
		RiskHotspots:      builder.riskHotspots,
		BoundaryCrossings: builder.boundaryCrossings,
		DecisionPath:      builder.decisionPath,
	}
}

type semanticGraphBuilder struct {
	nodes             []map[string]any
	edges             []map[string]any
	seenNodes         map[string]bool
	seenEdges         map[string]bool
	riskHotspots      []map[string]any
	boundaryCrossings []map[string]any
	decisionPath      []map[string]any
}

func (b *semanticGraphBuilder) addNode(id string, node map[string]any) {
	if b.seenNodes[id] {
		return
	}
	b.seenNodes[id] = true
	b.nodes = append(b.nodes, node)
}

func (b *semanticGraphBuilder) addEdge(from, to, edgeType, label, riskLevel string) {
	if from == "" || to == "" {
		return
	}
	id := from + "->" + to + ":" + edgeType
	if b.seenEdges[id] {
		return
	}
	b.seenEdges[id] = true
	b.edges = append(b.edges, map[string]any{
		"id":         id,
		"from":       from,
		"to":         to,
		"source":     from,
		"target":     to,
		"type":       edgeType,
		"label":      label,
		"risk_level": riskLevel,
	})
}

func normalizeSemanticStep(step SemanticGraphStep, index int) SemanticGraphStep {
	if strings.TrimSpace(step.StepID) == "" {
		step.StepID = fmt.Sprintf("step-%03d", firstPositive(step.StepIndex, index+1))
	}
	if step.StepIndex == 0 {
		step.StepIndex = index + 1
	}
	step.ToolName = fallbackText(step.ToolName, "unknown_tool")
	step.OperationType = fallbackText(step.OperationType, "inspect")
	step.ResourceType = fallbackText(step.ResourceType, "system")
	step.ResourcePath = fallbackText(step.ResourcePath, step.ResourceType)
	step.BoundaryLevel = fallbackText(step.BoundaryLevel, "unknown")
	step.PolicyDecision = fallbackText(step.PolicyDecision, policyNodeValue(step))
	step.Decision = fallbackText(step.Decision, "review")
	step.RiskLevel = fallbackText(step.RiskLevel, riskLevelForDecision(step.Decision))
	return step
}

func semanticNodeID(kind, value string) string {
	return kind + ":" + sanitizeGraphID(fallbackText(value, "unknown"))
}

func semanticResourceID(step SemanticGraphStep) string {
	return "resource:" + sanitizeGraphID(step.ResourceType+"|"+step.ResourcePath)
}

func semanticResourceLabel(step SemanticGraphStep) string {
	if step.ResourcePath != "" && step.ResourcePath != step.ResourceType {
		return step.ResourceType + " / " + step.ResourcePath
	}
	return step.ResourceType
}

func policyNodeValue(step SemanticGraphStep) string {
	if step.PolicyDecision != "" {
		return step.PolicyDecision
	}
	if step.AllowedByPolicy {
		return "allow"
	}
	return "review"
}

func decisionNodeValue(step SemanticGraphStep) string {
	return fallbackText(step.Decision, "review")
}

func semanticRiskLevel(step SemanticGraphStep) string {
	if step.RiskLevel != "" && step.RiskLevel != "unknown" {
		return step.RiskLevel
	}
	return riskLevelForDecision(step.Decision)
}

func semanticOperationRisk(operation string) string {
	value := strings.ToLower(operation)
	switch {
	case strings.Contains(value, "delete"), strings.Contains(value, "write"), strings.Contains(value, "execute"), strings.Contains(value, "modify"):
		return "high"
	case strings.Contains(value, "read"), strings.Contains(value, "inspect"), strings.Contains(value, "check"):
		return "low"
	default:
		return "medium"
	}
}

func semanticResourceRisk(step SemanticGraphStep) string {
	value := strings.ToLower(step.ResourceType + " " + step.ResourcePath)
	switch {
	case strings.Contains(value, "secret"), strings.Contains(value, "key"), strings.Contains(value, "audit"), strings.Contains(value, "log"), strings.Contains(value, "shell"):
		return "high"
	case strings.Contains(value, "service"), strings.Contains(value, "network"), strings.Contains(value, "port"):
		return "medium"
	default:
		return "low"
	}
}

func semanticBoundaryRisk(boundary string) string {
	value := strings.ToLower(boundary)
	switch value {
	case "dangerous", "critical", "privileged", "secret":
		return "high"
	case "sensitive", "medium", "restricted":
		return "medium"
	case "public", "low", "safe":
		return "low"
	default:
		return "medium"
	}
}

func semanticPolicyRisk(step SemanticGraphStep) string {
	if !step.AllowedByPolicy || strings.EqualFold(step.PolicyDecision, "deny") {
		return "high"
	}
	if strings.EqualFold(step.PolicyDecision, "review") {
		return "medium"
	}
	return "low"
}

func semanticHotspotSummary(step SemanticGraphStep) string {
	if step.RiskHint != "" {
		return step.RiskHint
	}
	if step.PolicyReason != "" {
		return step.PolicyReason
	}
	return fmt.Sprintf("%s reached %s boundary for %s", step.ToolName, step.BoundaryLevel, step.ResourceType)
}

func isBoundaryCrossing(boundary string) bool {
	level := semanticBoundaryRisk(boundary)
	return level == "high" || level == "medium"
}

func riskLevelForDecision(decision string) string {
	switch strings.ToLower(decision) {
	case "deny":
		return "high"
	case "review":
		return "medium"
	case "allow":
		return "low"
	default:
		return "unknown"
	}
}

func sanitizeGraphID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 1
}
