package reasoningtrace

import (
	"fmt"
	"strings"
)

// sensitiveKeys is a list of attribute keys that must be redacted.
var sensitiveKeys = []string{
	"api_key", "api-key", "apikey",
	"authorization", "auth",
	"bearer",
	"token",
	"password", "passwd",
	"secret",
	"credential",
	"private_key", "private-key", "privatekey",
	"access_key", "access-key", "accesskey",
	"secret_key", "secret-key", "secretkey",
}

// sensitiveValuePatterns are substrings that indicate a value contains sensitive data.
var sensitiveValuePatterns = []string{
	"bearer ",
	"Bearer ",
	"BEARER ",
	"sk-",        // OpenAI-style key
	"-----BEGIN", // Private key
}

// isSensitiveKey returns true if the key matches a sensitive key pattern.
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, sk := range sensitiveKeys {
		if lower == sk || strings.HasPrefix(lower, sk) || strings.HasSuffix(lower, sk) {
			return true
		}
	}
	return false
}

// containsSensitiveValue returns true if the value string contains sensitive content.
func containsSensitiveValue(value string) bool {
	lower := strings.ToLower(value)
	for _, pattern := range sensitiveValuePatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// SanitizeValue redacts a value if the key or value is sensitive.
func SanitizeValue(key string, value any) any {
	if isSensitiveKey(key) {
		return "[REDACTED]"
	}
	str, ok := value.(string)
	if !ok {
		return value
	}
	if containsSensitiveValue(str) {
		return "[REDACTED]"
	}
	return str
}

// SanitizeAttributes returns a copy of the attributes map with sensitive values redacted.
func SanitizeAttributes(attrs map[string]any) map[string]any {
	if attrs == nil {
		return nil
	}
	result := make(map[string]any, len(attrs))
	for k, v := range attrs {
		result[k] = SanitizeValue(k, v)
	}
	return result
}

// StringSlice safely converts any value to a comma-separated string slice representation.
func StringSlice(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ",")
}

// TruncatedSummary produces a safe summary string for reasoning traces.
// maxLen: maximum characters; if exceeded, truncates and appends "...".
func TruncatedSummary(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}
	// If it looks like log output, just say it was truncated.
	if len(value) > 200 {
		return fmt.Sprintf("[output truncated: %d bytes original]", len(value))
	}
	return value[:maxLen] + "..."
}
