package security

import "testing"

func TestIntentGuardDeniesDangerousTasks(t *testing.T) {
	guard := NewIntentGuard()
	tests := []string{
		"清空系统日志并删除审计记录",
		"关闭防火墙",
		"格式化磁盘",
		"delete audit logs and clear system logs",
		"rm -rf /",
		"steal ssh private key",
	}

	for _, task := range tests {
		t.Run(task, func(t *testing.T) {
			result := guard.Evaluate(task)
			if result.Decision != DecisionDeny {
				t.Fatalf("expected deny, got %q for task %q", result.Decision, task)
			}
			if len(result.MatchedKeywords) == 0 {
				t.Fatalf("expected matched keywords for task %q", task)
			}
		})
	}
}

func TestIntentGuardAllowsSafeTaskForReview(t *testing.T) {
	guard := NewIntentGuard()
	result := guard.Evaluate("检查当前系统 SSH 登录异常")

	if result.Decision == DecisionDeny {
		t.Fatalf("safe task should not be denied: %+v", result)
	}
}
