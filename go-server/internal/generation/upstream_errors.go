package generation

import (
	"net/http"
	"strings"
)

func isHTMLResponse(resp *http.Response, body []byte) bool {
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	text := strings.ToLower(strings.TrimSpace(string(body[:min(len(body), 80)])))
	return strings.Contains(contentType, "text/html") || strings.HasPrefix(text, "<!doctype") || strings.HasPrefix(text, "<html")
}

func cleanUpstreamError(payload any, text string) string {
	if payload != nil {
		if value := extractErrorMessage(payload); value != "" {
			return cleanPolicySuffix(value)
		}
	}
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return cleanPolicySuffix(trimLong(text, 500))
}

func extractErrorMessage(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	payload, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"message", "error_message", "detail"} {
		if text, ok := payload[key].(string); ok {
			return text
		}
	}
	if errValue, ok := payload["error"].(map[string]any); ok {
		if text, ok := errValue["message"].(string); ok {
			return text
		}
	}
	return ""
}

func cleanPolicySuffix(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, " / invalid_request_error / content_policy_violation")
	value = strings.TrimSuffix(value, " / content_policy_violation")
	return strings.TrimSpace(value)
}

func trimLong(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
