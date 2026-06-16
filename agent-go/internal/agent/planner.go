package agent

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type PlanStep struct {
	StepID   string         `json:"step_id"`
	ToolName string         `json:"tool_name"`
	Input    map[string]any `json:"input"`
	Reason   string         `json:"reason"`
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

type RuleBasedPlanner struct{}

func NewRuleBasedPlanner() RuleBasedPlanner {
	return RuleBasedPlanner{}
}

func (RuleBasedPlanner) Plan(ctx context.Context, task string) (Plan, error) {
	_ = ctx

	trimmed := strings.TrimSpace(task)
	if trimmed == "" {
		return Plan{}, fmt.Errorf("task is required")
	}

	normalized := normalizeTaskText(trimmed)
	switch {
	case isSSHAnomalyTask(normalized):
		return newPlan(trimmed, "ssh_anomaly_check", "Rule-based planner selected SSH anomaly diagnosis workflow", []PlanStep{
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
	case isServiceCheckTask(normalized):
		service := extractServiceName(normalized)
		return newPlan(trimmed, "service_check", "Rule-based planner selected service status workflow", []PlanStep{
			planStep("os_info", map[string]any{}, "Collect OS context before service diagnosis"),
			planStep("service_status", map[string]any{"service_name": service}, "Check target service status"),
		}), nil
	case isPortCheckTask(normalized):
		port := extractPort(normalized, 8080)
		return newPlan(trimmed, "port_check", "Rule-based planner selected port inspection workflow", []PlanStep{
			planStep("os_info", map[string]any{}, "Collect OS context before port inspection"),
			planStep("port_checker", map[string]any{"host": "127.0.0.1", "port": port}, "Check target local port"),
		}), nil
	default:
		return newPlan(trimmed, "system_overview", "Rule-based planner selected system overview workflow", []PlanStep{
			planStep("os_info", map[string]any{}, "Collect basic OS context"),
			planStep("port_checker", map[string]any{"host": "127.0.0.1", "port": 8080}, "Check default Go Agent port"),
		}), nil
	}
}

func newPlan(task string, scenario string, summary string, steps []PlanStep) Plan {
	for index := range steps {
		steps[index].StepID = fmt.Sprintf("plan-%03d", index+1)
		if steps[index].Input == nil {
			steps[index].Input = map[string]any{}
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
		"failed ssh login",
		"brute force",
		"suspicious login",
		"ssh failed login",
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
