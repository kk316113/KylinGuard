package report

import "strings"

func titleFor(input BuildInput) string {
	if isDangerousIntent(input) {
		return "KylinGuard Dangerous Intent Audit Report"
	}
	switch scenarioFromInput(input) {
	case "ssh_anomaly_check":
		return "KylinGuard SSH Login Anomaly Security Report"
	case "service_check":
		return "KylinGuard Service Status Security Report"
	case "port_check":
		return "KylinGuard Port Inspection Security Report"
	default:
		return "KylinGuard Security Audit Report"
	}
}

func whyRelevant(trace anyToolTrace) string {
	switch trace.ToolName {
	case "os_info":
		return "收集操作系统环境信息，用于确认诊断运行环境。"
	case "service_status":
		if strings.Contains(trace.ResourcePath, "sshd") {
			return "检查 sshd 服务状态，用于判断 SSH 登录入口是否存在。"
		}
		return "检查目标服务状态，用于判断安全运维对象是否正在运行。"
	case "port_checker":
		if strings.Contains(trace.ResourcePath, ":22") {
			return "检查 22 端口状态，用于确认 SSH 远程登录入口。"
		}
		return "检查目标端口监听状态，用于判断远程访问面是否存在。"
	case "log_reader":
		return "读取系统认证或安全日志，用于支持登录异常诊断。"
	case "ssh_login_analyzer":
		return "分析 SSH 认证日志中的失败登录、成功登录、无效用户和来源 IP。"
	default:
		return "该工具由 Planner 选择，用于完成用户请求。"
	}
}

func auditMeaning(trace anyToolTrace) string {
	switch trace.ToolName {
	case "os_info":
		return "该步骤属于公开系统信息读取，通常为低风险或 public 边界。"
	case "service_status":
		return "该步骤为只读服务状态检查，不执行服务修改操作。"
	case "port_checker":
		return "该步骤为只读网络端口检查，不修改网络配置。"
	case "log_reader":
		return "该步骤访问 system_log，通常属于 sensitive_system_resource，需要进入审计链路。"
	case "ssh_login_analyzer":
		return "该步骤对敏感认证日志进行安全分析，结果用于诊断但不直接作为最终审计裁决。"
	default:
		return "根据工具语义字段和 boundary_level 进行审计解释。"
	}
}

func accessReason(toolName string, resourceType string) string {
	switch toolName {
	case "log_reader":
		return "读取系统日志以支持用户请求的安全诊断。"
	case "ssh_login_analyzer":
		return "分析 SSH 认证日志以生成登录异常诊断。"
	default:
		if strings.Contains(resourceType, "credential") || strings.Contains(resourceType, "secret") {
			return "该资源可能包含敏感凭据，需要审计确认访问边界。"
		}
		return "该资源被工具调用链访问，因此纳入敏感资源审计。"
	}
}

func summaryFor(input BuildInput, report *SecurityReport) string {
	decision := fallback(input.Decision, "review")
	if isDangerousIntent(input) {
		summary := "该请求匹配危险运维意图，Intent Guard 在工具执行前完成拦截。系统未生成工具计划，未执行任何工具，也未访问系统资源，最终决策为 " + decision + "。This request was blocked before tool execution."
		return withRouteSummary(summary, input.Route)
	}

	scenario := scenarioFromInput(input)
	risk := fallback(report.RiskLevel, "unknown")
	if scenario == "ssh_anomaly_check" {
		summary := "本次任务被识别为 SSH 登录异常诊断。Agent 按计划检查系统信息、sshd 服务状态、22 端口、认证日志，并执行 SSH 登录异常分析。由于任务访问了敏感系统日志资源，TraceShield 对完整工具调用链进行了审计，最终决策为 " + decision + "。当前诊断风险等级为 " + risk + "。"
		return withRouteSummary(summary, input.Route)
	}

	summary := "Agent 按计划执行只读安全运维工具，并将语义化工具调用链提交给 audit-core-py / TraceShield 审计，最终决策为 " + decision + "。"
	return withRouteSummary(summary, input.Route)
}

func withRouteSummary(summary string, route string) string {
	if route == "eino-fallback" {
		return summary + " 该请求进入 Eino 实验接口，但当前 Eino Adapter 未启用，系统 fallback 到稳定 runtime。fallback 未绕过 intent_guard、Planner、工具语义 trace 或 TraceShield 审计链路。"
	}
	return summary
}

func diagnosisDescription(risk string) string {
	switch risk {
	case "low":
		return "SSH login anomaly diagnosis returned low risk based on the available authentication logs."
	case "medium":
		return "SSH login anomaly diagnosis found repeated failed login patterns and returned medium risk."
	case "high":
		return "SSH login anomaly diagnosis found high-frequency failed login patterns and returned high risk."
	default:
		return "SSH authentication logs were unavailable or insufficient; diagnosis risk level is unknown."
	}
}

func diagnosisSeverity(risk string) string {
	switch risk {
	case "high":
		return "high"
	case "medium":
		return "medium"
	case "low":
		return "low"
	default:
		return "info"
	}
}
