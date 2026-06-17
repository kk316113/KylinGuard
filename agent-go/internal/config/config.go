package config

import "os"

type Config struct {
	Addr               string
	AuditCoreURL       string
	EinoRuntimeEnabled bool
	EinoGraphEnabled   bool
	EinoLLMEnabled     bool
	EinoLLMProvider    string
	EinoLLMEndpoint    string
	EinoLLMModel       string
	EinoLLMAPIKey      string
}

func Load() Config {
	port := getenv("KYLIN_GUARD_AGENT_PORT", "8080")
	addr := os.Getenv("KYLIN_GUARD_AGENT_ADDR")
	if addr == "" {
		addr = ":" + port
	}

	return Config{
		Addr:               addr,
		AuditCoreURL:       getenv("AUDIT_CORE_URL", "http://127.0.0.1:8001"),
		EinoRuntimeEnabled: getenvBool("EINO_RUNTIME_ENABLED", true),
		EinoGraphEnabled:   getenvBool("EINO_GRAPH_ENABLED", true),
		EinoLLMEnabled:     getenvBool("EINO_LLM_ENABLED", getenvBool("EINO_ENABLED", false)),
		EinoLLMProvider:    getenv("EINO_LLM_PROVIDER", "deterministic"),
		EinoLLMEndpoint:    os.Getenv("EINO_LLM_ENDPOINT"),
		EinoLLMModel:       os.Getenv("EINO_LLM_MODEL"),
		EinoLLMAPIKey:      os.Getenv("EINO_LLM_API_KEY"),
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
