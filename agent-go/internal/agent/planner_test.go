package agent

import (
	"context"
	"testing"
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
	assertPlanTools(t, plan, []string{"os_info", "service_status", "port_checker", "log_reader"})

	if plan.Steps[1].Input["service_name"] != "sshd" {
		t.Fatalf("expected sshd service, got %#v", plan.Steps[1].Input["service_name"])
	}
	if plan.Steps[2].Input["port"] != 22 {
		t.Fatalf("expected port 22, got %#v", plan.Steps[2].Input["port"])
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
	}
}
