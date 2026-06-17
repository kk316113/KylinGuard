package agent

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"kylin-guard-agent/agent-go/internal/tools"
)

type PlanStep struct {
	StepID          string         `json:"step_id"`
	ToolName        string         `json:"tool_name"`
	Input           map[string]any `json:"input"`
	Reason          string         `json:"reason"`
	ToolCategory    string         `json:"tool_category,omitempty"`
	RiskLevel       string         `json:"risk_level,omitempty"`
	PermissionScope string         `json:"permission_scope,omitempty"`
}

type Plan struct {
	Task     string     `json:"task"`
	Scenario string     `json:"scenario"`
	Summary  string     `json:"summary"`
	Steps    []PlanStep `json:"steps"`
}

type Planner interface {
	Plan(ctx context.Context, task string) (Plan, error)
}

type RuleBasedPlanner struct {
	registry *tools.Registry
}

func NewRuleBasedPlanner() RuleBasedPlanner {
	return NewRuleBasedPlannerWithRegistry(tools.NewDefaultRegistry())
}

func NewRuleBasedPlannerWithRegistry(registry *tools.Registry) RuleBasedPlanner {
	if registry == nil {
		registry = tools.NewDefaultRegistry()
	}
	return RuleBasedPlanner{registry: registry}
}

func (p RuleBasedPlanner) Plan(ctx context.Context, task string) (Plan, error) {
	_ = ctx

	trimmed := strings.TrimSpace(task)
	if trimmed == "" {
		return Plan{}, fmt.Errorf("task is required")
	}

	normalized := normalizeTaskText(trimmed)
	switch {
	case isSSHAnomalyTask(normalized):
		return p.newPlan(trimmed, "ssh_anomaly_check", "Rule-based planner selected SSH anomaly diagnosis workflow", []PlanStep{
			planStep("os_info", map[string]any{}, "Collect OS context before diagnosis"),
			planStep("service_status", map[string]any{"service_name": "sshd"}, "Check sshd service status"),
			planStep("port_checker", map[string]any{"host": "127.0.0.1", "port": 22}, "Check whether local SSH port is reachable"),
			planStep("log_reader", map[string]any{
				"paths":   []string{"/var/log/secure", "/var/log/auth.log"},
				"lines":   100,
				"purpose": "ssh_login_anomaly_check",
			}, "Read recent SSH authentication logs if available"),
			planStep("ssh_login_analyzer", map[string]any{
				"paths": []string{"/var/log/secure", "/var/log/auth.log"},
				"lines": 200,
			}, "Analyze SSH authentication failures, accepted logins, and top failed source IPs"),
		}), nil
	case isSystemSecurityOverviewTask(normalized):
		return p.newPlan(trimmed, "system_security_overview", "Rule-based planner selected system security overview workflow", []PlanStep{
			planStep("os_info", map[string]any{}, "Collect basic OS context"),
			planStep("resource_usage_checker", map[string]any{}, "Check system load and memory usage"),
			planStep("disk_memory_checker", map[string]any{"include_tmpfs": false}, "Check disk usage and memory summary"),
			planStep("network_connection_inspector", map[string]any{"state": "LISTEN", "limit": 100}, "Check network listening ports"),
			planStep("service_status", map[string]any{"service_name": "sshd"}, "Check sshd service status"),
			planStep("process_inspector", map[string]any{"name": "sshd", "limit": 20}, "Inspect sshd process status"),
			planStep("journalctl_reader", map[string]any{"service_name": "sshd", "lines": 100}, "Read recent sshd journal logs"),
		}), nil
	case isSystemResourceCheckTask(normalized):
		return p.newPlan(trimmed, "system_resource_check", "Rule-based planner selected system resource check workflow", []PlanStep{
			planStep("os_info", map[string]any{}, "Collect OS context before resource check"),
			planStep("resource_usage_checker", map[string]any{}, "Check system load and memory usage"),
			planStep("disk_memory_checker", map[string]any{"include_tmpfs": false}, "Check disk usage and memory summary"),
		}), nil
	case isNetworkConnectionCheckTask(normalized):
		return p.newPlan(trimmed, "network_connection_check", "Rule-based planner selected network connection check workflow", []PlanStep{
			planStep("network_connection_inspector", map[string]any{"state": "LISTEN", "limit": 100}, "Inspect listening network ports"),
			planStep("port_checker", map[string]any{"host": "127.0.0.1", "port": 22}, "Check whether local SSH port is reachable"),
		}), nil
	case isProcessHealthCheckTask(normalized):
		return p.newPlan(trimmed, "process_health_check", "Rule-based planner selected process health check workflow", []PlanStep{
			planStep("process_inspector", map[string]any{"name": "sshd", "limit": 20}, "Inspect sshd process status"),
			planStep("service_status", map[string]any{"service_name": "sshd"}, "Check sshd service status"),
		}), nil
	case isJournalLogCheckTask(normalized):
		return p.newPlan(trimmed, "journal_log_check", "Rule-based planner selected journal log check workflow", []PlanStep{
			planStep("journalctl_reader", map[string]any{"service_name": "sshd", "lines": 100}, "Read recent sshd journal logs"),
		}), nil
	case isServiceCheckTask(normalized):
		service := extractServiceName(normalized)
		return p.newPlan(trimmed, "service_check", "Rule-based planner selected service status workflow", []PlanStep{
			planStep("os_info", map[string]any{}, "Collect OS context before service diagnosis"),
			planStep("service_status", map[string]any{"service_name": service}, "Check target service status"),
		}), nil
	case isPortCheckTask(normalized):
		port := extractPort(normalized, 8080)
		return p.newPlan(trimmed, "port_check", "Rule-based planner selected port inspection workflow", []PlanStep{
			planStep("os_info", map[string]any{}, "Collect OS context before port inspection"),
			planStep("port_checker", map[string]any{"host": "127.0.0.1", "port": port}, "Check target local port"),
		}), nil
	default:
		return p.newPlan(trimmed, "system_overview", "Rule-based planner selected system overview workflow", []PlanStep{
			planStep("os_info", map[string]any{}, "Collect basic OS context"),
			planStep("port_checker", map[string]any{"host": "127.0.0.1", "port": 8080}, "Check default Go Agent port"),
		}), nil
	}
}

func (p RuleBasedPlanner) newPlan(task string, scenario string, summary string, steps []PlanStep) Plan {
	for index := range steps {
		steps[index].StepID = fmt.Sprintf("plan-%03d", index+1)
		if steps[index].Input == nil {
			steps[index].Input = map[string]any{}
		}
		if p.registry != nil {
			if metadata, ok := p.registry.GetTool(steps[index].ToolName); ok && metadata.Enabled {
				steps[index].ToolCategory = metadata.Category
				steps[index].RiskLevel = metadata.RiskLevel
				steps[index].PermissionScope = metadata.PermissionScope
			} else {
				steps[index].Reason = steps[index].Reason + "; tool metadata unavailable or disabled"
			}
		}
	}
	return Plan{
		Task:     task,
		Scenario: scenario,
		Summary:  summary,
		Steps:    steps,
	}
}

func planStep(toolName string, input map[string]any, reason string) PlanStep {
	return PlanStep{
		ToolName: toolName,
		Input:    input,
		Reason:   reason,
	}
}

func normalizeTaskText(task string) string {
	lower := strings.ToLower(task)
	return strings.Join(strings.Fields(lower), " ")
}

func isSSHAnomalyTask(task string) bool {
	return containsAny(task, []string{
		"ssh 登录异常",
		"登录失败",
		"暴力破解",
		"异常登录",
		"远程登录",
		"ssh login anomaly",
		"ssh login anomalies",
		"ssh auth log",
		"ssh authentication",
		"failed ssh login",
		"ssh failed login",
		"brute force",
		"suspicious login",
		"suspicious ssh",
		"ssh login security",
		"inspect ssh",
		"analyze ssh",
		"analyze failed ssh",
	})
}

func isServiceCheckTask(task string) bool {
	return containsAny(task, []string{
		"服务状态",
		"检查服务",
		"systemd",
		"sshd",
		"nginx",
		"docker",
		"service status",
		"check service",
	})
}

func isPortCheckTask(task string) bool {
	return containsAny(task, []string{
		"端口",
		"监听",
		"开放端口",
		"port",
		"listen",
		"open port",
	})
}

func isSystemSecurityOverviewTask(task string) bool {
	return containsAny(task, []string{
		"系统安全巡检",
		"系统安全检查",
		"系统状态巡检",
		"安全巡检",
		"system security overview",
		"system security check",
		"security overview",
	})
}

func isSystemResourceCheckTask(task string) bool {
	return containsAny(task, []string{
		"系统资源使用",
		"查看系统负载",
		"查看 cpu",
		"内存状态",
		"检查内存",
		"检查负载",
		"资源使用情况",
		"资源使用",
		"系统负载",
		"system resource check",
		"system resource usage",
		"check resource usage",
		"check system resource",
		"check cpu and memory",
		"check cpu memory",
		"disk and memory usage",
		"check system load",
		"check system performance",
		"check resource",
		"load check",
		"memory check",
		"resource usage check",
		"inspect disk",
		"inspect resource",
		"analyze resource",
	})
}

func isNetworkConnectionCheckTask(task string) bool {
	return containsAny(task, []string{
		"网络连接",
		"监听端口",
		"查看监听",
		"异常端口",
		"端口暴露",
		"network connection",
		"listening port",
		"check connection",
	})
}

func isProcessHealthCheckTask(task string) bool {
	return containsAny(task, []string{
		"进程状态",
		"检查进程",
		"查看进程",
		"sshd 进程",
		"process status",
		"process check",
		"check process",
	})
}

func isJournalLogCheckTask(task string) bool {
	return containsAny(task, []string{
		"最近日志",
		"读取日志",
		"查看日志",
		"服务日志",
		"journal log",
		"journalctl",
		"read log",
		"check log",
		"systemd 日志",
	}) && !containsAny(task, []string{
		"ssh 登录异常",
		"登录失败",
		"暴力破解",
		"login anomaly",
		"failed login",
		"brute force",
		"删除",
		"清除",
		"delete",
		"clear",
	})
}

func containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func extractServiceName(task string) string {
	switch {
	case strings.Contains(task, "nginx"):
		return "nginx"
	case strings.Contains(task, "docker"):
		return "docker"
	case strings.Contains(task, "sshd") || strings.Contains(task, "ssh"):
		return "sshd"
	default:
		return "sshd"
	}
}

var portPattern = regexp.MustCompile(`\b([1-9][0-9]{0,4})\b`)

func extractPort(task string, fallback int) int {
	matches := portPattern.FindAllStringSubmatch(task, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		port, err := strconv.Atoi(match[1])
		if err == nil && port >= 1 && port <= 65535 {
			return port
		}
	}
	return fallback
}
