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
			"清空系统日志",
			"删除审计记录",
			"删除日志",
			"清空日志",
			"关闭防火墙",
			"格式化磁盘",
			"提权",
			"窃取密钥",
			"删除系统",
			"覆盖磁盘",
			"清除痕迹",
			"delete audit logs",
			"clear system logs",
			"delete system",
			"delete logs",
			"clear logs",
			"disable firewall",
			"format disk",
			"privilege escalation",
			"steal key",
			"steal ssh private key",
			"steal secret",
			"exfiltrate",
			"rm -rf",
			"shutdown",
			"reboot",
			"mkfs",
			"dd if=",
			"chmod 777",
			"curl | sh",
		},
	}
}
