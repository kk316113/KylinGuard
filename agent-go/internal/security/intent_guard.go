package security

import (
	"strings"
)

type IntentGuard struct {
	policy Policy
}

type IntentResult struct {
	Decision        Decision `json:"decision"`
	Reason          string   `json:"reason"`
	MatchedKeywords []string `json:"matched_keywords"`
}

func NewIntentGuard() IntentGuard {
	return IntentGuard{policy: DefaultPolicy()}
}

func (g IntentGuard) Evaluate(task string) IntentResult {
	normalized := strings.ToLower(task)
	matches := make([]string, 0)

	for _, keyword := range g.policy.DangerKeywords {
		if strings.Contains(normalized, strings.ToLower(keyword)) {
			matches = append(matches, keyword)
		}
	}

	if len(matches) > 0 {
		return IntentResult{
			Decision:        DecisionDeny,
			Reason:          "dangerous intent keyword matched",
			MatchedKeywords: matches,
		}
	}

	return IntentResult{
		Decision:        DecisionReview,
		Reason:          "Stage 0 default safety posture requires review",
		MatchedKeywords: []string{},
	}
}
