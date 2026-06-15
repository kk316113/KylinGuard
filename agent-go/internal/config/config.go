package config

import "os"

type Config struct {
	Addr         string
	AuditCoreURL string
	EinoEnabled  bool
}

func Load() Config {
	port := getenv("KYLIN_GUARD_AGENT_PORT", "8080")
	addr := os.Getenv("KYLIN_GUARD_AGENT_ADDR")
	if addr == "" {
		addr = ":" + port
	}

	return Config{
		Addr:         addr,
		AuditCoreURL: getenv("AUDIT_CORE_URL", "http://127.0.0.1:8001"),
		EinoEnabled:  getenvBool("EINO_ENABLED", false),
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getenvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "TRUE", "True", "yes", "YES", "on", "ON":
		return true
	default:
		return false
	}
}
