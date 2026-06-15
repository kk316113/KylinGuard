package config

import "os"

type Config struct {
	Addr         string
	AuditCoreURL string
}

func Load() Config {
	port := getenv("KYLIN_GUARD_AGENT_PORT", "8080")
	addr := os.Getenv("KYLIN_GUARD_AGENT_ADDR")
	if addr == "" {
		addr = ":" + port
	}

	return Config{
		Addr:         addr,
		AuditCoreURL: getenv("AUDIT_CORE_URL", "http://127.0.0.1:8090"),
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
