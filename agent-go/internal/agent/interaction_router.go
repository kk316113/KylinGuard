package agent

import (
	"strings"

	"kylin-guard-agent/agent-go/internal/auditclient"
	"kylin-guard-agent/agent-go/internal/logtrace"
)

const (
	InteractionTypeChat        = "chat"
	InteractionTypeAgentRun    = "agent_run"
	InteractionTypeSafeRefusal = "safe_refusal"
	InteractionTypeClarify     = "clarify"

	RouterSourceLLMRouter                 = "llm_router"
	RouterSourceSafetyGuard               = "safety_guard"
	RouterSourceDeterministicConservative = "deterministic_conservative"

	RouterConfidenceHigh   = "high"
	RouterConfidenceMedium = "medium"
	RouterConfidenceLow    = "low"

	AgentModeChatOnly = "chat_only"
)

type InteractionRoute struct {
	InteractionType    string `json:"interaction_type"`
	RouterSource       string `json:"router_source"`
	Confidence         string `json:"confidence"`
	NeedsToolExecution bool   `json:"needs_tool_execution"`
	Reason             string `json:"reason,omitempty"`
}

func NewSafetyRefusalRoute(reason string) InteractionRoute {
	return InteractionRoute{
		InteractionType:    InteractionTypeSafeRefusal,
		RouterSource:       RouterSourceSafetyGuard,
		Confidence:         RouterConfidenceHigh,
		NeedsToolExecution: false,
		Reason:             reason,
	}
}

// ConservativeRoute is only a deterministic fallback for environments without
// a remote LLM router. It must not select tools or become fixed workflow logic.
func ConservativeRoute(task string) InteractionRoute {
	text := strings.TrimSpace(strings.ToLower(task))
	if text == "" {
		return clarifyRoute("empty input")
	}
	if isExplicitDemoOpsRequest(text) {
		return InteractionRoute{
			InteractionType:    InteractionTypeAgentRun,
			RouterSource:       RouterSourceDeterministicConservative,
			Confidence:         RouterConfidenceMedium,
			NeedsToolExecution: true,
			Reason:             "concrete operations task; deterministic fallback allows Agent Loop to plan next_action",
		}
	}
	return InteractionRoute{
		InteractionType:    InteractionTypeChat,
		RouterSource:       RouterSourceDeterministicConservative,
		Confidence:         RouterConfidenceLow,
		NeedsToolExecution: false,
		Reason:             "non-operational input defaults to safe chat when no semantic LLM router is available",
	}
}

func NewChatOnlyResponse(task string, route InteractionRoute) RunResponse {
	answer := "你好，我是 KylinGuard。有什么可以帮你？"
	return NewChatOnlyResponseWithAnswer(task, route, answer)
}

func NewChatOnlyResponseWithAnswer(task string, route InteractionRoute, answer string) RunResponse {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		answer = "你好，我是 KylinGuard。有什么可以帮你？"
	}
	return newNonToolResponse(task, route, AgentModeChatOnly, "KylinGuard", answer, []string{})
}

func NewClarifyResponse(task string, route InteractionRoute) RunResponse {
	answer := "我还不确定你希望我处理什么。你可以继续描述，我会先理解你的请求，再决定是否需要调用工具。"
	return newNonToolResponse(task, route, AgentModeChatOnly, "需要更多信息", answer, []string{})
}

func NewSafeRefusalResponse(task string, route InteractionRoute) RunResponse {
	answer := "不建议执行该操作。该请求涉及高风险或可能破坏安全追踪的行为。在没有明确审批、备份和合规流程前，我不会调用系统工具执行这类操作。"
	resp := newNonToolResponse(task, route, AgentModeChatOnly, "安全建议", answer, []string{
		"不要执行删除、清空、关闭安全防护或规避审计的操作。",
		"如确需处理，请先走变更审批、备份和审计留痕流程。",
	})
	resp.Decision = "deny"
	resp.RunStatus = RunStatusBlocked
	if resp.UserMessage != nil {
		resp.UserMessage.Status = RunStatusBlocked
	}
	return resp
}

func newNonToolResponse(task string, route InteractionRoute, agentMode string, title string, answer string, next []string) RunResponse {
	resp := RunResponse{
		Task:               task,
		InteractionType:    route.InteractionType,
		RouterSource:       route.RouterSource,
		RouterConfidence:   route.Confidence,
		NeedsToolExecution: route.NeedsToolExecution,
		RouterReason:       route.Reason,
		AgentMode:          agentMode,
		Decision:           "allow",
		Summary:            answer,
		RunStatus:          RunStatusCompleted,
		FinalAnswer:        answer,
		UserMessage: &UserMessage{
			Title:        title,
			Answer:       answer,
			Status:       RunStatusCompleted,
			WhatIChecked: []string{},
			KeyFindings:  []string{},
			NextSteps:    next,
		},
		AgentSteps:  []map[string]any{},
		ToolTrace:   []logtrace.ToolTrace{},
		AuditResult: auditclient.Result{},
	}
	AttachScenarioWorkspaceMetadata(&resp, task, RunStatusCompleted)
	resp.InteractionType = route.InteractionType
	resp.RouterSource = route.RouterSource
	resp.RouterConfidence = route.Confidence
	resp.NeedsToolExecution = route.NeedsToolExecution
	resp.RouterReason = route.Reason
	return resp
}

func clarifyRoute(reason string) InteractionRoute {
	return InteractionRoute{
		InteractionType:    InteractionTypeClarify,
		RouterSource:       RouterSourceDeterministicConservative,
		Confidence:         RouterConfidenceLow,
		NeedsToolExecution: false,
		Reason:             reason,
	}
}

func isExplicitDemoOpsRequest(text string) bool {
	hasTroubleshootingIntent := strings.Contains(text, "帮我看看") ||
		strings.Contains(text, "排查") ||
		strings.Contains(text, "检查") ||
		strings.Contains(text, "诊断") ||
		strings.Contains(text, "定位") ||
		strings.Contains(text, "diagnose") ||
		strings.Contains(text, "check") ||
		strings.Contains(text, "troubleshoot")
	if !hasTroubleshootingIntent {
		return false
	}
	return strings.Contains(text, "ssh") ||
		strings.Contains(text, "连不上") ||
		strings.Contains(text, "登录") ||
		strings.Contains(text, "服务") ||
		strings.Contains(text, "端口") ||
		strings.Contains(text, "访问不了") ||
		strings.Contains(text, "卡") ||
		strings.Contains(text, "慢") ||
		strings.Contains(text, "cpu") ||
		strings.Contains(text, "内存") ||
		strings.Contains(text, "磁盘") ||
		strings.Contains(text, "日志") ||
		strings.Contains(text, "service") ||
		strings.Contains(text, "port") ||
		strings.Contains(text, "slow") ||
		strings.Contains(text, "load") ||
		strings.Contains(text, "log")
}
