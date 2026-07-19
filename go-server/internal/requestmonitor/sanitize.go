package requestmonitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"unicode"
)

const (
	maxCapturedBodyBytes = 256 * 1024
	maxStoredStringRunes = 4000
	redactedValue        = "[已脱敏]"
)

var sensitiveKeys = map[string]struct{}{
	"access_token": {}, "apikey": {}, "api_key": {}, "authorization": {},
	"cookie": {}, "key": {}, "key_plain": {}, "password": {}, "passwd": {},
	"refresh_token": {}, "secret": {}, "token": {},
}

type CapturedRequest struct {
	QueryParams json.RawMessage
	BodyParams  json.RawMessage
	SourceIP    string
	SourceHost  string
	Origin      string
	Referer     string
	UserAgent   string
}

func ShouldRecord(req *http.Request) bool {
	if req == nil || req.Method == http.MethodOptions {
		return false
	}
	path := req.URL.Path
	if path == "/api/health" || path == "/api/admin/request-monitor" {
		return false
	}
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/oauth/")
}

func Capture(req *http.Request) CapturedRequest {
	origin := sanitizeSourceURL(req.Header.Get("Origin"))
	referer := sanitizeSourceURL(req.Header.Get("Referer"))
	host := sourceHost(origin)
	if host == "" {
		host = sourceHost(referer)
	}
	return CapturedRequest{
		QueryParams: sanitizeValues(req.URL.Query()),
		BodyParams:  captureBody(req),
		SourceIP:    sourceIP(req),
		SourceHost:  host,
		Origin:      origin,
		Referer:     referer,
		UserAgent:   truncateString(strings.TrimSpace(req.UserAgent())),
	}
}

func sanitizeValues(values url.Values) json.RawMessage {
	result := make(map[string]any, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if isSensitiveKey(key) {
			result[key] = redactedValue
			continue
		}
		items := values[key]
		if len(items) == 1 {
			result[key] = sanitizeString(items[0])
			continue
		}
		clean := make([]string, 0, min(len(items), 50))
		for index, value := range items {
			if index >= 50 {
				break
			}
			clean = append(clean, sanitizeString(value))
		}
		result[key] = clean
	}
	return marshalJSON(result)
}

func captureBody(req *http.Request) json.RawMessage {
	if req.Body == nil || req.Body == http.NoBody {
		return marshalJSON(map[string]any{})
	}
	mediaType, _, _ := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if strings.HasPrefix(mediaType, "multipart/") {
		return marshalJSON(map[string]any{
			"_summary":      "multipart 请求，文件内容未记录",
			"contentType":   mediaType,
			"contentLength": req.ContentLength,
		})
	}

	original := req.Body
	prefix, err := io.ReadAll(io.LimitReader(original, maxCapturedBodyBytes+1))
	req.Body = &restoredBody{Reader: io.MultiReader(bytes.NewReader(prefix), original), closer: original}
	if err != nil {
		return marshalJSON(map[string]any{"_summary": "请求体读取失败"})
	}
	if len(prefix) > maxCapturedBodyBytes {
		return marshalJSON(map[string]any{
			"_summary":      "请求体过大，内容未记录",
			"contentType":   mediaType,
			"contentLength": req.ContentLength,
		})
	}
	trimmed := bytes.TrimSpace(prefix)
	if len(trimmed) == 0 {
		return marshalJSON(map[string]any{})
	}

	if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") || json.Valid(trimmed) {
		var value any
		if json.Unmarshal(trimmed, &value) == nil {
			return marshalJSON(sanitizeValue("", value, 0))
		}
	}
	if mediaType == "application/x-www-form-urlencoded" {
		if values, parseErr := url.ParseQuery(string(trimmed)); parseErr == nil {
			return sanitizeValues(values)
		}
	}
	return marshalJSON(map[string]any{
		"_summary":    "非 JSON 请求体",
		"contentType": mediaType,
		"text":        sanitizeString(string(trimmed)),
	})
}

type restoredBody struct {
	io.Reader
	closer io.Closer
}

func (body *restoredBody) Close() error {
	return body.closer.Close()
}

func sanitizeValue(key string, value any, depth int) any {
	if isSensitiveKey(key) {
		return redactedValue
	}
	if depth > 8 {
		return "[嵌套层级过深，已省略]"
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, min(len(typed), 100))
		keys := make([]string, 0, len(typed))
		for childKey := range typed {
			keys = append(keys, childKey)
		}
		sort.Strings(keys)
		for index, childKey := range keys {
			if index >= 100 {
				result["_truncated"] = fmt.Sprintf("另有 %d 个字段已省略", len(keys)-100)
				break
			}
			result[childKey] = sanitizeValue(childKey, typed[childKey], depth+1)
		}
		return result
	case []any:
		limit := min(len(typed), 50)
		result := make([]any, 0, limit+1)
		for index := 0; index < limit; index++ {
			result = append(result, sanitizeValue(key, typed[index], depth+1))
		}
		if len(typed) > limit {
			result = append(result, fmt.Sprintf("另有 %d 项已省略", len(typed)-limit))
		}
		return result
	case string:
		return sanitizeString(typed)
	default:
		return value
	}
}

func sanitizeString(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "data:image/") {
		return fmt.Sprintf("[图片数据已省略，共 %d 字符]", len(value))
	}
	if len(value) >= 512 && looksLikeBase64(value) {
		return fmt.Sprintf("[Base64 数据已省略，共 %d 字符]", len(value))
	}
	return truncateString(value)
}

func truncateString(value string) string {
	runes := []rune(value)
	if len(runes) <= maxStoredStringRunes {
		return value
	}
	return string(runes[:maxStoredStringRunes]) + fmt.Sprintf("…[另有 %d 字符已省略]", len(runes)-maxStoredStringRunes)
}

func looksLikeBase64(value string) bool {
	valid := 0
	for _, char := range value {
		if unicode.IsSpace(char) {
			continue
		}
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '+' || char == '/' || char == '=' || char == '-' || char == '_' {
			valid++
			continue
		}
		return false
	}
	return valid >= 512
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if _, exists := sensitiveKeys[normalized]; exists {
		return true
	}
	return strings.HasSuffix(normalized, "_password") || strings.HasSuffix(normalized, "_secret") || strings.HasSuffix(normalized, "_token") || strings.HasSuffix(normalized, "_api_key")
}

func sanitizeSourceURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return truncateString(raw)
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return truncateString(parsed.String())
}

func sourceHost(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

func sourceIP(req *http.Request) string {
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(req.Header.Get(header))
		if value != "" {
			return strings.TrimSpace(strings.Split(value, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(req.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(req.RemoteAddr)
}

func marshalJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{"_summary":"参数序列化失败"}`)
	}
	return encoded
}
