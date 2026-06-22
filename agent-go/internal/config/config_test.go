package config

import "testing"

func TestLoadUsesOpenAICompatibleEnvironmentForRealLLM(t *testing.T) {
	clearLLMEnvironment(t)
	t.Setenv("OPENAI_COMPATIBLE_API_KEY", "test-key")
	t.Setenv("OPENAI_COMPATIBLE_BASE_URL", "https://api.deepseek.com")
	t.Setenv("OPENAI_COMPATIBLE_MODEL", "deepseek-v4-flash")

	cfg := Load()

	if !cfg.EinoLLMEnabled {
		t.Fatal("expected a configured API key to enable the remote LLM")
	}
	if cfg.EinoLLMProvider != "openai_compatible" {
		t.Fatalf("expected openai_compatible provider, got %q", cfg.EinoLLMProvider)
	}
	if cfg.EinoLLMEndpoint != "https://api.deepseek.com" || cfg.EinoLLMModel != "deepseek-v4-flash" {
		t.Fatalf("unexpected endpoint/model: %q %q", cfg.EinoLLMEndpoint, cfg.EinoLLMModel)
	}
	if cfg.EinoLLMAPIKey != "test-key" {
		t.Fatal("expected compatible API key to be forwarded")
	}
}

func TestLoadPrefersExplicitEinoConfiguration(t *testing.T) {
	clearLLMEnvironment(t)
	t.Setenv("OPENAI_COMPATIBLE_API_KEY", "compatible-key")
	t.Setenv("EINO_LLM_API_KEY", "eino-key")
	t.Setenv("EINO_LLM_PROVIDER", "deepseek")
	t.Setenv("EINO_LLM_ENDPOINT", "https://example.invalid/v1")
	t.Setenv("EINO_LLM_MODEL", "custom-model")
	t.Setenv("EINO_LLM_ENABLED", "false")

	cfg := Load()

	if cfg.EinoLLMEnabled {
		t.Fatal("expected explicit EINO_LLM_ENABLED=false to win")
	}
	if cfg.EinoLLMProvider != "deepseek" || cfg.EinoLLMEndpoint != "https://example.invalid/v1" || cfg.EinoLLMModel != "custom-model" {
		t.Fatalf("explicit Eino configuration was not preserved: %#v", cfg)
	}
	if cfg.EinoLLMAPIKey != "eino-key" {
		t.Fatal("expected explicit Eino API key to win")
	}
}

func TestLoadWithoutAPIKeyKeepsDeterministicMode(t *testing.T) {
	clearLLMEnvironment(t)

	cfg := Load()

	if cfg.EinoLLMEnabled || cfg.EinoLLMProvider != "deterministic" || cfg.EinoLLMAPIKey != "" {
		t.Fatalf("expected deterministic mode without a key, got %#v", cfg)
	}
}

func TestLoadTrimsRemoteLLMEnvironmentValues(t *testing.T) {
	clearLLMEnvironment(t)
	t.Setenv("OPENAI_COMPATIBLE_API_KEY", "  test-key\r\n")
	t.Setenv("OPENAI_COMPATIBLE_BASE_URL", " https://api.deepseek.com/ \n")
	t.Setenv("OPENAI_COMPATIBLE_MODEL", " deepseek-v4-flash ")

	cfg := Load()
	if cfg.EinoLLMAPIKey != "test-key" || cfg.EinoLLMEndpoint != "https://api.deepseek.com/" || cfg.EinoLLMModel != "deepseek-v4-flash" {
		t.Fatalf("expected trimmed LLM configuration, got key length=%d endpoint=%q model=%q", len(cfg.EinoLLMAPIKey), cfg.EinoLLMEndpoint, cfg.EinoLLMModel)
	}
}

func clearLLMEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"EINO_LLM_ENABLED",
		"EINO_ENABLED",
		"EINO_LLM_PROVIDER",
		"EINO_LLM_ENDPOINT",
		"EINO_LLM_MODEL",
		"EINO_LLM_API_KEY",
		"OPENAI_COMPATIBLE_API_KEY",
		"OPENAI_COMPATIBLE_BASE_URL",
		"OPENAI_COMPATIBLE_MODEL",
		"OPENAI_API_KEY",
		"DEEPSEEK_API_KEY",
	} {
		t.Setenv(key, "")
	}
}
