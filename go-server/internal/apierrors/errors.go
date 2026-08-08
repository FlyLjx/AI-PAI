package apierrors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Details is the normalized error envelope shared by the upstream adapter,
// OpenAI-compatible responses, and API usage accounting.
type Details struct {
	StatusCode   int    `json:"-"`
	Message      string `json:"message,omitempty"`
	Title        string `json:"title,omitempty"`
	Type         string `json:"type,omitempty"`
	Code         string `json:"code,omitempty"`
	Category     string `json:"category,omitempty"`
	Retryable    bool   `json:"retryable"`
	Action       string `json:"action,omitempty"`
	Hint         string `json:"hint,omitempty"`
	RequestID    string `json:"request_id,omitempty"`
	RetryableSet bool   `json:"-"`
}

// Parse extracts the v0.1.67 error envelope while accepting the older
// message/detail shapes used by compatible upstreams.
func Parse(status int, payload any, body []byte) Details {
	details := Details{StatusCode: status}
	if value, ok := errorPayload(payload); ok {
		details.Message = stringValue(value["message"])
		if details.Message == "" {
			details.Message = stringValue(value["error_message"])
		}
		if details.Message == "" {
			details.Message = stringValue(value["detail"])
		}
		details.Title = stringValue(value["title"])
		details.Type = stringValue(value["type"])
		details.Code = stringValue(value["code"])
		details.Category = stringValue(value["category"])
		details.Action = stringValue(value["action"])
		details.Hint = stringValue(value["hint"])
		details.RequestID = stringValue(value["request_id"])
		if retryable, exists := value["retryable"].(bool); exists {
			details.Retryable = retryable
			details.RetryableSet = true
		} else {
			details.Retryable = IsCountedFailureStatus(status)
		}
	}
	if details.Message == "" {
		details.Message = bodyText(body)
	}
	details.Message = CleanMessage(details.Message)
	Normalize(&details)
	return details
}

func errorPayload(payload any) (map[string]any, bool) {
	value, ok := payload.(map[string]any)
	if !ok {
		return nil, false
	}
	if nested, ok := value["error"].(map[string]any); ok {
		return nested, true
	}
	return value, true
}

func bodyText(body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" || isHTMLText(text) {
		return ""
	}
	return trimLong(text, 500)
}

func isHTMLText(value string) bool {
	text := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(text, "<!doctype") || strings.HasPrefix(text, "<html")
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

// Normalize fills the stable fields used by clients when an upstream only
// returns a status code or a legacy message.
func Normalize(details *Details) {
	if details == nil {
		return
	}
	if details.StatusCode == 0 {
		details.StatusCode = http.StatusBadGateway
	}
	if details.Message == "" {
		details.Message = fmt.Sprintf("上游接口调用失败：HTTP %d", details.StatusCode)
	}
	if details.Type == "" {
		details.Type = defaultType(details.StatusCode)
	}
	if details.Code == "" {
		details.Code = DefaultCode(details.StatusCode)
	}
	if details.Category == "" {
		details.Category = defaultCategory(details.StatusCode)
	}
	if details.Title == "" {
		details.Title = defaultTitle(details.Code, details.StatusCode)
	}
	if details.Action == "" {
		details.Action = defaultAction(details.Code, details.StatusCode)
	}
	if details.Hint == "" {
		details.Hint = defaultHint(details.Code, details.StatusCode)
	}
	if !details.RetryableSet && (details.StatusCode == http.StatusBadGateway || details.StatusCode == http.StatusTooManyRequests) {
		// The upstream envelope marks these responses retryable. This default
		// also covers HTML gateway pages and legacy upstream responses.
		details.Retryable = true
	}
}

func (details *Details) UnmarshalJSON(data []byte) error {
	type detailsAlias Details
	var decoded detailsAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*details = Details(decoded)
	_, details.RetryableSet = fields["retryable"]
	return nil
}

func defaultType(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "invalid_api_key"
	case http.StatusPaymentRequired:
		return "insufficient_quota"
	case http.StatusBadRequest, http.StatusNotFound, http.StatusForbidden, http.StatusRequestTimeout,
		http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity, http.StatusTooManyRequests:
		return "invalid_request_error"
	default:
		return "api_error"
	}
}

func defaultCategory(status int) string {
	if status >= 500 {
		return "upstream"
	}
	if status >= 400 {
		return "request"
	}
	return "api"
}

func defaultTitle(code string, status int) string {
	switch code {
	case "reference_image_required":
		return "需要参考图"
	case "reference_image_invalid":
		return "参考图无效"
	case "content_policy_violation":
		return "内容不符合要求"
	case "image_generation_failed", "image_session_failed", "image_result_unavailable", "image_upload_failed", "upstream_service_error":
		return "上游生图服务异常"
	case "image_generation_stalled", "oai_image_generation_timeout", "upstream_rate_limited", "image_quota_exhausted", "account_pool_unavailable", "task_queue_full":
		return "上游生图资源繁忙"
	case "upstream_timeout", "image_upload_timeout", "image_preparation_timeout":
		return "上游服务超时"
	case "api_key_invalid":
		return "API Key 无效"
	case "endpoint_not_allowed", "model_not_allowed":
		return "接口权限不足"
	case "client_quota_exhausted":
		return "调用额度不足"
	}
	if status >= 500 {
		return "上游服务异常"
	}
	return "请求处理失败"
}

func defaultAction(code string, status int) string {
	if code == "reference_image_required" {
		return "upload_reference_image"
	}
	if IsCountedFailureStatus(status) || status >= 500 {
		return "retry"
	}
	return "check_request"
}

func defaultHint(code string, status int) string {
	if code == "reference_image_required" {
		return "请上传缩略图或参考图后重新提交任务"
	}
	if IsCountedFailureStatus(status) || status >= 500 {
		return "请稍后重试"
	}
	return "请检查请求参数"
}

// DefaultCode maps local gateway errors to the stable client-facing code set.
func DefaultCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_request"
	case http.StatusUnauthorized:
		return "api_key_invalid"
	case http.StatusForbidden:
		return "endpoint_not_allowed"
	case http.StatusRequestTimeout:
		return "upstream_timeout"
	case http.StatusPaymentRequired:
		return "client_quota_exhausted"
	case http.StatusPreconditionRequired:
		return "interactive_challenge_required"
	case http.StatusTooManyRequests:
		return "upstream_rate_limited"
	case http.StatusBadGateway:
		return "upstream_service_error"
	case http.StatusServiceUnavailable:
		return "upstream_service_busy"
	case http.StatusGatewayTimeout:
		return "upstream_timeout"
	case 499:
		return "request_canceled"
	default:
		return "api_error"
	}
}

func CleanMessage(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, " / invalid_request_error / content_policy_violation")
	value = strings.TrimSuffix(value, " / content_policy_violation")
	return trimLong(strings.TrimSpace(value), 500)
}

func trimLong(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

// IsCountedFailureStatus is the single success-rate failure rule used by API
// access logs and request monitoring.
func IsCountedFailureStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway
}

func IsSuccessStatus(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}
