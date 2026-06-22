package security

import (
	"strings"
	"unicode"
)

const (
	ThreatTypeDangerousIntent = "dangerous_intent"
	ThreatTypePromptInjection = "prompt_injection"
)

type IntentGuard struct {
	policy Policy
}

type IntentResult struct {
	Decision        Decision `json:"decision"`
	Reason          string   `json:"reason"`
	MatchedKeywords []string `json:"matched_keywords"`
	ThreatType      string   `json:"threat_type,omitempty"`
}

func NewIntentGuard() IntentGuard {
	return IntentGuard{policy: DefaultPolicy()}
}

func (g IntentGuard) Evaluate(task string) IntentResult {
	normalized := normalizeIntentText(task)
	injectionMatches := matchPatterns(normalized, g.policy.PromptInjectionPatterns)
	if len(injectionMatches) > 0 {
		return IntentResult{
			Decision:        DecisionDeny,
			Reason:          "prompt injection attempt detected",
			MatchedKeywords: injectionMatches,
			ThreatType:      ThreatTypePromptInjection,
		}
	}

	matches := matchPatterns(normalized, g.policy.DangerKeywords)

	if len(matches) > 0 {
		return IntentResult{
			Decision:        DecisionDeny,
			Reason:          "dangerous intent keyword matched",
			MatchedKeywords: matches,
			ThreatType:      ThreatTypeDangerousIntent,
		}
	}

	return IntentResult{
		Decision:        DecisionReview,
		Reason:          "default safety posture requires review",
		MatchedKeywords: []string{},
	}
}

func normalizeIntentText(value string) string {
	var normalized strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			normalized.WriteRune(r)
		}
	}
	return normalized.String()
}

func matchPatterns(normalized string, patterns []string) []string {
	matches := make([]string, 0)
	for _, pattern := range patterns {
		if strings.Contains(normalized, normalizeIntentText(pattern)) {
			matches = append(matches, pattern)
		}
	}
	return matches
}

// NeutralizeUntrustedText prevents tool observations or arguments from being
// replayed as instructions in the next LLM turn. The original evidence stays
// in the tool trace; only the model-facing copy is replaced. Whole-value
// replacement prevents split or obfuscated instructions from surviving.
func NeutralizeUntrustedText(value string) (string, bool) {
	policy := DefaultPolicy()
	if len(matchPatterns(normalizeIntentText(value), policy.PromptInjectionPatterns)) == 0 {
		return value, false
	}
	return "[UNTRUSTED_INSTRUCTION_REDACTED: potential prompt injection retained only in audit trace]", true
}
