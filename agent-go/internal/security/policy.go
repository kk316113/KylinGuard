package security

type Decision string

const (
	DecisionAllow  Decision = "allow"
	DecisionReview Decision = "review"
	DecisionDeny   Decision = "deny"
)

type Policy struct {
	DangerKeywords []string
}

func DefaultPolicy() Policy {
	return Policy{
		DangerKeywords: []string{
			"删除系统",
			"清空日志",
			"关闭防火墙",
			"格式化磁盘",
			"提权",
			"窃取密钥",
			"delete system",
			"clear logs",
			"disable firewall",
			"format disk",
			"privilege escalation",
			"steal key",
			"steal secret",
			"exfiltrate",
		},
	}
}
