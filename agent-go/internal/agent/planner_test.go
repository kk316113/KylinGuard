package agent

import (
	"context"
	"testing"

	"kylin-guard-agent/agent-go/internal/tools"
)

func TestRuleBasedPlannerSSHAnomaly(t *testing.T) {
	planner := NewRuleBasedPlanner()

	plan, err := planner.Plan(context.Background(), "检查当前系统 SSH 登录异常")
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if plan.Scenario != "ssh_anomaly_check" {
		t.Fatalf("expected ssh_anomaly_check, got %q", plan.Scenario)
	}
	assertPlanTools(t, plan, []string{"os_info", "service_status", "port_checker", "log_reader", "ssh_login_analyzer"})

	if plan.Steps[1].Input["service_name"] != "sshd" {
		t.Fatalf("expected sshd service, got %#v", plan.Steps[1].Input["service_name"])
	}
	if plan.Steps[2].Input["port"] != 22 {
		t.Fatalf("expected port 22, got %#v", plan.Steps[2].Input["port"])
	}
	if plan.Steps[4].Input["lines"] != 200 {
		t.Fatalf("expected ssh_login_analyzer lines=200, got %#v", plan.Steps[4].Input["lines"])
	}
}

func TestRuleBasedPlannerServiceCheck(t *testing.T) {
	planner := NewRuleBasedPlanner()

	plan, err := planner.Plan(context.Background(), "检查 nginx 服务状态")
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if plan.Scenario != "service_check" {
		t.Fatalf("expected service_check, got %q", plan.Scenario)
	}
	assertPlanTools(t, plan, []string{"os_info", "service_status"})
	if plan.Steps[1].Input["service_name"] != "nginx" {
		t.Fatalf("expected nginx service, got %#v", plan.Steps[1].Input["service_name"])
	}
}

func TestRuleBasedPlannerPortCheck(t *testing.T) {
	planner := NewRuleBasedPlanner()

	plan, err := planner.Plan(context.Background(), "检查 8080 端口是否开放")
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if plan.Scenario != "port_check" {
		t.Fatalf("expected port_check, got %q", plan.Scenario)
	}
	assertPlanTools(t, plan, []string{"os_info", "port_checker"})
	if plan.Steps[1].Input["port"] != 8080 {
		t.Fatalf("expected port 8080, got %#v", plan.Steps[1].Input["port"])
	}
}

func TestRuleBasedPlannerSSHAnomalyEnglish(t *testing.T) {
	planner := NewRuleBasedPlanner()

	tests := []string{
		"check SSH login anomaly",
		"inspect SSH auth logs",
		"analyze failed ssh login attempts",
		"analyze SSH login failures",
		"check suspicious SSH logins",
		"check ssh login security",
	}
	for _, task := range tests {
		plan, err := planner.Plan(context.Background(), task)
		if err != nil {
			t.Fatalf("plan returned error for %q: %v", task, err)
		}
		if plan.Scenario != "ssh_anomaly_check" {
			t.Fatalf("expected ssh_anomaly_check for %q, got %q", task, plan.Scenario)
		}
		assertPlanTools(t, plan, []string{"os_info", "service_status", "port_checker", "log_reader", "ssh_login_analyzer"})
	}
}

func TestRuleBasedPlannerDefaultSystemOverview(t *testing.T) {
	planner := NewRuleBasedPlanner()

	plan, err := planner.Plan(context.Background(), "检查当前系统状态")
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if plan.Scenario != "system_overview" {
		t.Fatalf("expected system_overview, got %q", plan.Scenario)
	}
	assertPlanTools(t, plan, []string{"os_info", "port_checker"})
}

func assertPlanTools(t *testing.T, plan Plan, expected []string) {
	t.Helper()
	registry := tools.NewDefaultRegistry()
	if len(plan.Steps) != len(expected) {
		t.Fatalf("expected %d steps, got %d: %#v", len(expected), len(plan.Steps), plan.Steps)
	}
	for index, toolName := range expected {
		if plan.Steps[index].ToolName != toolName {
			t.Fatalf("step %d expected tool %q, got %q", index, toolName, plan.Steps[index].ToolName)
		}
		if plan.Steps[index].StepID == "" {
			t.Fatalf("step %d missing step_id", index)
		}
		metadata, ok := registry.GetTool(plan.Steps[index].ToolName)
		if !ok {
			t.Fatalf("step %d tool %q missing from registry", index, plan.Steps[index].ToolName)
		}
		if !metadata.Enabled {
			t.Fatalf("step %d tool %q is disabled", index, plan.Steps[index].ToolName)
		}
		if plan.Steps[index].ToolCategory == "" {
			t.Fatalf("step %d missing tool_category", index)
		}
		if plan.Steps[index].RiskLevel == "" {
			t.Fatalf("step %d missing risk_level", index)
		}
		if plan.Steps[index].PermissionScope == "" {
			t.Fatalf("step %d missing permission_scope", index)
		}
	}
}
