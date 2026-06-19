package agent

import (
	"fmt"
	"strings"
)

// AttachUserMessage makes every Agent response usable as a normal assistant
// answer. It only summarizes completed execution; it does not route tools.
func AttachUserMessage(resp *RunResponse) {
	if resp == nil {
		return
	}
	status := normalizeRunStatus(resp.RunStatus)
	if status == "" {
		status = RunStatusCompleted
	}

	checked := checkedItems(resp)
	findings := keyFindings(resp)
	nextSteps := nextSteps(resp)
	title := userMessageTitle(resp.Decision, status)
	answer := strings.TrimSpace(resp.FinalAnswer)
	if answer == "" {
		answer = buildUserFacingAnswer(resp, status, checked, findings, nextSteps)
	}
	if answer == "" {
		answer = "我已完成本次运维任务的基础检查，并整理了可查看的执行步骤、工具证据和安全审计结果。"
	}

	resp.FinalAnswer = answer
	if strings.TrimSpace(resp.Summary) == "" || isTechnicalSummary(resp.Summary) {
		resp.Summary = answer
	}
	resp.UserMessage = &UserMessage{
		Title:        title,
		Answer:       answer,
		Status:       status,
		WhatIChecked: checked,
		KeyFindings:  findings,
		NextSteps:    nextSteps,
	}
}

func userMessageTitle(decision string, status string) string {
	if status == RunStatusBlocked || decision == "deny" {
		return "安全建议"
	}
	if status == RunStatusFailed {
		return "任务未完成"
	}
	if status == RunStatusPartial {
		return "部分完成"
	}
	return "排查结论"
}

func buildUserFacingAnswer(resp *RunResponse, status string, checked []string, findings []string, next []string) string {
	if status == RunStatusBlocked || resp.Decision == "deny" {
		return "不建议执行该操作。该请求涉及删除、清空或破坏审计线索的高风险意图，可能影响安全追踪、问题复盘和合规审计。本次请求已在执行任何工具前被安全策略拦截，因此没有进行真实危险操作。"
	}
	if status == RunStatusFailed {
		msg := "这次任务没有执行成功。可能是后端服务、网络代理或模型配置异常。"
		if resp.AuditResult.Message != "" {
			msg += " 当前错误信息：" + resp.AuditResult.Message
		}
		return msg
	}

	parts := []string{"我已按安全受控的方式完成本次运维检查。"}
	if len(checked) > 0 {
		parts = append(parts, "我重点查看了"+strings.Join(checked, "、")+"。")
	}
	if len(findings) > 0 {
		parts = append(parts, "当前可见结论是："+strings.Join(findings, "；")+"。")
	}
	if len(next) > 0 {
		parts = append(parts, "建议下一步："+strings.Join(next, "；")+"。")
	}
	return strings.Join(parts, "")
}

func checkedItems(resp *RunResponse) []string {
	items := []string{}
	seen := map[string]bool{}
	add := func(item string) {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			return
		}
		seen[item] = true
		items = append(items, item)
	}
	for _, trace := range resp.ToolTrace {
		switch trace.ToolName {
		case "service_status":
			add("服务状态")
		case "port_checker":
			add("端口连通性")
		case "ssh_login_analyzer":
			add("SSH 登录日志")
		case "log_reader", "journalctl_reader":
			add("系统日志线索")
		case "resource_usage_checker", "disk_memory_checker":
			add("CPU、内存和磁盘资源")
		case "network_connection_inspector":
			add("网络连接状态")
		case "process_inspector":
			add("进程状态")
		case "os_info":
			add("系统基础信息")
		default:
			if trace.ResourceType != "" {
				add(trace.ResourceType)
			}
		}
	}
	if len(items) == 0 && resp.Decision == "deny" {
		add("安全意图")
	}
	return items
}

func keyFindings(resp *RunResponse) []string {
	findings := []string{}
	if resp.Decision != "" {
		switch resp.Decision {
		case "deny":
			findings = append(findings, "安全策略判定该请求不应执行")
		case "review":
			findings = append(findings, "结果需要人工复核后再处置")
		case "allow":
			findings = append(findings, "未发现需要立即拦截的高风险动作")
		}
	}
	if resp.Diagnosis != nil {
		for _, finding := range resp.Diagnosis.Findings {
			findings = append(findings, truncateUserText(finding, 96))
			if len(findings) >= 3 {
				return findings
			}
		}
	}
	for _, trace := range resp.ToolTrace {
		if trace.OutputSummary != "" {
			findings = append(findings, fmt.Sprintf("%s：%s", trace.ToolName, truncateUserText(trace.OutputSummary, 96)))
			if len(findings) >= 3 {
				return findings
			}
		}
	}
	if len(findings) == 0 && resp.AuditResult.Message != "" {
		findings = append(findings, truncateUserText(resp.AuditResult.Message, 120))
	}
	return findings
}

func nextSteps(resp *RunResponse) []string {
	if resp.Decision == "deny" {
		return []string{
			"不要清空或删除审计日志",
			"保留现场证据，改用只读方式导出必要日志",
			"如确需处理日志，请先走变更审批和备份流程",
		}
	}
	if resp.Diagnosis != nil && len(resp.Diagnosis.Recommendations) > 0 {
		result := make([]string, 0, len(resp.Diagnosis.Recommendations))
		for _, item := range resp.Diagnosis.Recommendations {
			result = append(result, truncateUserText(item, 100))
			if len(result) >= 3 {
				return result
			}
		}
		return result
	}
	if len(resp.ToolTrace) > 0 {
		return []string{
			"结合右侧步骤和证据查看具体检查结果",
			"确认服务、端口、日志和权限配置是否符合预期",
		}
	}
	return []string{"补充具体故障现象、服务名、端口或日志位置后继续定位"}
}

func isTechnicalSummary(summary string) bool {
	value := strings.ToLower(strings.TrimSpace(summary))
	return value == "" ||
		value == "agent run completed" ||
		value == "request denied by intent guard" ||
		strings.Contains(value, "graph runtime executed") ||
		strings.Contains(value, "fallback-mock")
}

func truncateUserText(text string, max int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max]) + "..."
}
