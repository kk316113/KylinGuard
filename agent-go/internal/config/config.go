package config

import (
	"os"
	"strings"
)

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
	apiKey := firstNonEmpty(
		os.Getenv("EINO_LLM_API_KEY"),
		os.Getenv("OPENAI_COMPATIBLE_API_KEY"),
		os.Getenv("OPENAI_API_KEY"),
		os.Getenv("DEEPSEEK_API_KEY"),
	)
	remoteConfigured := apiKey != ""
	providerFallback := "deterministic"
	if remoteConfigured {
		providerFallback = "openai_compatible"
	}
	endpoint := firstNonEmpty(os.Getenv("EINO_LLM_ENDPOINT"), os.Getenv("OPENAI_COMPATIBLE_BASE_URL"))
	model := firstNonEmpty(os.Getenv("EINO_LLM_MODEL"), os.Getenv("OPENAI_COMPATIBLE_MODEL"))
	if remoteConfigured {
		if endpoint == "" {
			endpoint = "https://api.deepseek.com"
		}
		if model == "" {
			model = "deepseek-v4-flash"
		}
	}

	return Config{
		Addr:               addr,
		AuditCoreURL:       getenv("AUDIT_CORE_URL", "http://127.0.0.1:8001"),
		EinoRuntimeEnabled: getenvBool("EINO_RUNTIME_ENABLED", true),
		EinoGraphEnabled:   getenvBool("EINO_GRAPH_ENABLED", true),
		EinoLLMEnabled:     getenvBool("EINO_LLM_ENABLED", getenvBool("EINO_ENABLED", remoteConfigured)),
		EinoLLMProvider:    getenv("EINO_LLM_PROVIDER", providerFallback),
		EinoLLMEndpoint:    endpoint,
		EinoLLMModel:       model,
		EinoLLMAPIKey:      apiKey,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
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
