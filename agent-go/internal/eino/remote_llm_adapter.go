package eino

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"kylin-guard-agent/agent-go/internal/agent"
	"kylin-guard-agent/agent-go/internal/security"
	"kylin-guard-agent/agent-go/internal/tools"
)

const (
	defaultLLMTimeout   = 45 * time.Second
	chatCompletionsPath = "/v1/chat/completions"
)

// RemoteLLMAdapter calls an OpenAI-compatible API for structured tool plan generation.
type RemoteLLMAdapter struct {
	config   ChatModelAdapterConfig
	registry *tools.Registry
	client   *http.Client
}

// LLMResponse holds metadata about the LLM call outcome.
type LLMResponse struct {
	ToolPlan       *ToolPlan
	RawOutput      string
	UsedFallback   bool
	FallbackReason string
	Provider       string
	Model          string
	Error          error
}

// NewRemoteLLMAdapter creates a new RemoteLLMAdapter.
// It validates the config: if LLM is enabled but API key or endpoint is missing,
// the adapter will still be created but GenerateToolCalls will return a clear error.
func NewRemoteLLMAdapter(config ChatModelAdapterConfig, registry *tools.Registry) *RemoteLLMAdapter {
	config.Provider = strings.TrimSpace(config.Provider)
	config.Endpoint = strings.TrimSpace(config.Endpoint)
	config.Model = strings.TrimSpace(config.Model)
	config.APIKey = strings.TrimSpace(config.APIKey)
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = int(defaultLLMTimeout.Seconds())
	}
	return &RemoteLLMAdapter{
		config:   config,
		registry: registry,
		client:   &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
}

// ValidateLLMConfig checks whether the LLM config is usable for a real remote call.
// Returns an empty string if valid, or a descriptive reason if not.
func ValidateLLMConfig(provider string, endpoint string, apiKey string) string {
	if provider == "" || provider == "deterministic" {
		return "LLM provider is not configured for remote calls"
	}
	if apiKey == "" {
		return "EINO_LLM_API_KEY is not set"
	}
	if !validBearerCredential(apiKey) {
		return "LLM API key contains characters that are invalid in an Authorization header"
	}
	if provider == "deepseek" && endpoint == "" {
		return "EINO_LLM_ENDPOINT is required for deepseek provider"
	}
	if provider == "openai_compatible" && endpoint == "" {
		return "EINO_LLM_ENDPOINT is required for openai_compatible provider"
	}
	return ""
}

func validBearerCredential(value string) bool {
	for _, char := range value {
		if char < 0x21 || char > 0x7e {
			return false
		}
	}
	return value != ""
}

// Name returns the adapter identifier.
func (a *RemoteLLMAdapter) Name() string {
	if a.config.AdapterName != "" {
		return a.config.AdapterName
	}
	return fmt.Sprintf("remote-llm-%s", a.config.Provider)
}

// Provider returns the LLM provider type.
func (a *RemoteLLMAdapter) Provider() string {
	return a.config.Provider
}

// GenerateToolCalls calls the remote LLM API and returns structured tool calls.
func (a *RemoteLLMAdapter) GenerateToolCalls(ctx context.Context, task string, toolDefs []tools.ToolMetadata) ([]ToolCall, agent.Plan, error) {
	if intent := security.NewIntentGuard().Evaluate(task); intent.Decision == security.DecisionDeny {
		return nil, agent.Plan{}, fmt.Errorf("remote LLM request denied by intent guard: %s", intent.Reason)
	}
	// Validate config before making any request.
	if reason := ValidateLLMConfig(a.config.Provider, a.config.Endpoint, a.config.APIKey); reason != "" {
		return nil, agent.Plan{}, fmt.Errorf("remote LLM config invalid: %s", reason)
	}

	endpoint := a.resolveEndpoint()
	prompt := a.buildSystemPrompt()
	messages := []map[string]any{
		{"role": "system", "content": prompt},
		{"role": "user", "content": task},
	}

	body := map[string]any{
		"model":       a.config.Model,
		"messages":    messages,
		"temperature": 0.1,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, agent.Plan{}, fmt.Errorf("failed to marshal LLM request: %w", err)
	}

	respBody, err := a.doHTTPRequestWithRetry(ctx, endpoint, payload, 2)
	if err != nil {
		return nil, agent.Plan{}, err
	}

	return a.parseLLMResponse(respBody, task)
}

func (a *RemoteLLMAdapter) parseLLMResponse(respBody []byte, task string) ([]ToolCall, agent.Plan, error) {
	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, agent.Plan{}, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}
	if len(openAIResp.Choices) == 0 {
		return nil, agent.Plan{}, fmt.Errorf("LLM returned zero choices")
	}
	content := strings.TrimSpace(openAIResp.Choices[0].Message.Content)
	if content == "" {
		return nil, agent.Plan{}, fmt.Errorf("LLM returned empty content")
	}

	toolPlan, err := ParseToolPlanJSON([]byte(content))
	if err != nil {
		return nil, agent.Plan{}, fmt.Errorf("LLM output is not valid JSON tool plan: %w", err)
	}

	validation := ValidateToolPlan(toolPlan, a.registry)
	if !validation.Valid {
		return nil, agent.Plan{}, fmt.Errorf("tool plan validation failed: %s", validation.FallbackReason)
	}

	calls := make([]ToolCall, 0, len(validation.ValidTools))
	validIndex := 0
	for _, item := range toolPlan.ToolPlan {
		toolName := strings.TrimSpace(item.ToolName)
		// Check if this tool was rejected.
		rejected := false
		for _, rejectedTool := range validation.RejectedTools {
			if toolName == rejectedTool {
				rejected = true
				break
			}
		}
		if rejected {
			continue
		}
		if item.Arguments == nil {
			item.Arguments = map[string]any{}
		}
		validIndex++
		call := ToolCall{
			ID:       fmt.Sprintf("llm-call-%03d", validIndex),
			ToolName: toolName,
			Input:    item.Arguments,
			Reason:   item.Reason,
		}
		calls = append(calls, call)
	}

	scenario := toolPlan.Scenario
	if scenario == "" {
		scenario = "llm_generated"
	}
	plan := planFromToolCalls(task, scenario, "Remote LLM adapter selected tool plan", calls, a.registry)

	return calls, plan, nil
}

func (a *RemoteLLMAdapter) buildSystemPrompt() string {
	toolDefs := BuildToolDefsForPrompt(a.registry)
	toolDefsJSON, _ := json.MarshalIndent(toolDefs, "", "  ")

	return fmt.Sprintf(`You are a security operations assistant that generates structured tool plans. Rules:

1. Output ONLY valid JSON. No markdown, no code fences, no explanation.
2. You cannot execute shell commands. You cannot output shell commands.
3. You cannot bypass security policies.
4. You can only select tools from the allowed tool registry below.
5. You cannot decide the final allow/deny — the system decides.
6. If the task involves sensitive logs (auth logs, journal logs, system logs), set requires_review=true.
7. If the task is dangerous (deletion, firewall, kill, privilege escalation), return empty tool_plan.

Output JSON schema:
{
  "scenario": "string — e.g. ssh_anomaly_check, system_resource_check, system_security_overview",
  "intent": "string — brief intent",
  "tool_plan": [
    {
      "tool_name": "string — must be from allowed tool list",
      "reason": "string",
      "arguments": {}
    }
  ],
  "risk_hint": "low|medium|high",
  "requires_review": false,
  "user_explanation": "string"
}

Allowed tools:
%s`, string(toolDefsJSON))
}

// callChatCompletions sends a chat completion request and returns the response content.
func (a *RemoteLLMAdapter) callChatCompletions(ctx context.Context, messages []map[string]any) (string, error) {
	if a.config.APIKey == "" {
		return "", fmt.Errorf("remote LLM API key is not configured")
	}
	endpoint := a.resolveEndpoint()
	body := map[string]any{
		"model":       a.config.Model,
		"messages":    messages,
		"temperature": 0.1,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal LLM request: %w", err)
	}
	respBody, err := a.doHTTPRequestWithRetry(ctx, endpoint, payload, 2)
	if err != nil {
		return "", err
	}
	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return "", fmt.Errorf("failed to parse OpenAI response: %w", err)
	}
	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned zero choices")
	}
	return strings.TrimSpace(openAIResp.Choices[0].Message.Content), nil
}

// doHTTPRequestWithRetry sends an HTTP POST request with retry logic for transient failures.
// maxRetries is the maximum number of additional attempts (so total attempts = maxRetries + 1).
// Retries on: network errors, HTTP 429 (rate limit), HTTP 502/503/504 (server errors).
// Does NOT retry on: 4xx client errors (except 429), context cancellation.
func (a *RemoteLLMAdapter) doHTTPRequestWithRetry(ctx context.Context, endpoint string, payload []byte, maxRetries int) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s, ...
			delay := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("failed to create LLM request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+a.config.APIKey)

		resp, err := a.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("LLM API call cancelled: %w", ctx.Err())
			}
			lastErr = fmt.Errorf("LLM API call failed: %w", err)
			continue
		}

		respBody, err := readLimitedLLMBody(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if isRetryableLLMStatus(resp.StatusCode) {
			lastErr = fmt.Errorf("LLM API returned HTTP %d (attempt %d/%d): %s", resp.StatusCode, attempt+1, maxRetries+1, truncateLLMString(string(respBody), 200))
			continue
		}

		// Non-retryable HTTP error (4xx except 429).
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("LLM API returned HTTP %d: %s", resp.StatusCode, truncateLLMString(string(respBody), 200))
		}

		return respBody, nil
	}
	return nil, fmt.Errorf("LLM API call failed after %d attempts: %w", maxRetries+1, lastErr)
}

func isRetryableLLMStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func readLimitedLLMBody(body io.Reader) ([]byte, error) {
	const maxBody = 1 << 20
	data, err := io.ReadAll(io.LimitReader(body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read LLM response: %w", err)
	}
	if len(data) > maxBody {
		return nil, fmt.Errorf("LLM response exceeds %d bytes", maxBody)
	}
	return data, nil
}

func (a *RemoteLLMAdapter) resolveEndpoint() string {
	ep := a.config.Endpoint
	if ep == "" {
		return ""
	}
	// Remove trailing slash for clean concatenation.
	ep = strings.TrimRight(ep, "/")
	// If it already ends with /chat/completions, return as-is.
	if strings.HasSuffix(ep, "/chat/completions") {
		return ep
	}
	// If it's a base /v1 path (e.g., https://api.openai.com/v1), append.
	if strings.HasSuffix(ep, "/v1") {
		return ep + "/chat/completions"
	}
	// Check if the URL contains /v1/chat/completions anywhere (e.g., https://api.deepseek.com/v1/chat/completions).
	if strings.Contains(ep, "/v1/chat/completions") || strings.Contains(ep, "/chat/completions") {
		return ep
	}
	// Default: treat as base URL, append /v1/chat/completions.
	return ep + chatCompletionsPath
}

func truncateLLMString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
