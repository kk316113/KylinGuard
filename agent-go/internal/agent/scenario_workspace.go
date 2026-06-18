package agent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	RunStatusCompleted = "completed"
	RunStatusBlocked   = "blocked"
	RunStatusFailed    = "failed"
	RunStatusPartial   = "partial"
)

// AttachScenarioWorkspaceMetadata adds response-level task session metadata.
//
// This classification is display-only and must not route tools or determine workflow.
func AttachScenarioWorkspaceMetadata(resp *RunResponse, task string, status string) {
	if resp == nil {
		return
	}
	resp.TaskID = newTaskID()
	resp.SceneType = ClassifySceneTypeForDisplay(task)
	resp.SceneSummary = buildSceneSummary(task, resp.TaskUnderstanding)
	resp.RunStatus = normalizeRunStatus(status)
	resp.CreatedAt = time.Now().UTC().Format(time.RFC3339)
}

// ClassifySceneTypeForDisplay is UI metadata only. It is deliberately kept out of
// planner and Agent Loop logic so it cannot become scenario workflow routing.
func ClassifySceneTypeForDisplay(task string) string {
	t := strings.ToLower(strings.TrimSpace(task))
	if t == "" {
		return "unknown"
	}
	if containsAnySceneToken(t, []string{"安全吗", "清空", "删除", "审计", "日志", "权限", "风险", "safe", "risk", "audit", "log", "delete", "clear", "permission"}) {
		return "security_check"
	}
	if containsAnySceneToken(t, []string{"卡", "慢", "cpu", "内存", "磁盘", "负载", "memory", "disk", "load", "slow"}) {
		return "system_health"
	}
	if containsAnySceneToken(t, []string{"服务", "端口", "访问不了", "启动", "挂了", "service", "port", "unreachable", "down", "start"}) {
		return "service_recovery"
	}
	if containsAnySceneToken(t, []string{"ssh", "连不上", "登录", "异常", "login", "connect"}) {
		return "diagnosis"
	}
	if containsAnySceneToken(t, []string{"合规", "基线", "整改", "compliance", "baseline"}) {
		return "compliance_review"
	}
	return "unknown"
}

func buildSceneSummary(task string, understanding map[string]any) string {
	if understanding != nil {
		if goal, ok := understanding["user_goal"].(string); ok && strings.TrimSpace(goal) != "" {
			return truncateSummary(goal)
		}
	}
	return truncateSummary(task)
}

func truncateSummary(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return "Operations task"
	}
	runes := []rune(text)
	if len(runes) <= 80 {
		return text
	}
	return string(runes[:80]) + "..."
}

func normalizeRunStatus(status string) string {
	switch status {
	case RunStatusCompleted, RunStatusBlocked, RunStatusFailed, RunStatusPartial:
		return status
	default:
		return RunStatusCompleted
	}
}

func containsAnySceneToken(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func newTaskID() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return "kg-" + hex.EncodeToString(buf[:])
	}
	return fmt.Sprintf("kg-%d", time.Now().UnixNano())
}
