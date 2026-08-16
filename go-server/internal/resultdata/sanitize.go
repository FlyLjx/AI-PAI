package resultdata

import (
	"encoding/json"
	"strings"
)

// WithoutInlineImages removes embedded image bytes while preserving URLs,
// usage metadata, errors, and other small result fields.
func WithoutInlineImages(value any) any {
	cleaned, _ := withoutInlineImages(value)
	return cleaned
}

// MarshalWithoutInlineImages serializes a result after recursively removing
// inline image bytes. The JSON round trip also handles structs and typed
// slices, which cannot be traversed by WithoutInlineImages directly.
func MarshalWithoutInlineImages(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	cleaned, _, err := SanitizeResultJSON(string(raw))
	if err != nil {
		return nil, err
	}
	if cleaned == nil {
		return nil, nil
	}
	return *cleaned, nil
}

func withoutInlineImages(value any) (any, bool) {
	switch item := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(item))
		changed := false
		for key, child := range item {
			normalizedKey := strings.ToLower(strings.TrimSpace(key))
			if normalizedKey == "b64_json" || normalizedKey == "base64" {
				changed = true
				continue
			}
			cleaned, childChanged := withoutInlineImages(child)
			changed = changed || childChanged
			if cleaned != nil {
				result[key] = cleaned
			}
		}
		if len(result) == 0 {
			return nil, changed
		}
		return result, changed
	case []any:
		result := make([]any, 0, len(item))
		changed := false
		for _, child := range item {
			cleaned, childChanged := withoutInlineImages(child)
			changed = changed || childChanged
			if cleaned != nil {
				result = append(result, cleaned)
			}
		}
		if len(result) == 0 {
			return nil, changed
		}
		return result, changed
	case string:
		if isInlineImage(item) {
			return nil, true
		}
		return item, false
	default:
		return value, false
	}
}

func SanitizeResultJSON(raw string) (*string, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, false, nil
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, false, err
	}
	cleaned, changed := withoutInlineImages(value)
	if !changed {
		return &raw, false, nil
	}
	if cleaned == nil {
		return nil, true, nil
	}
	encoded, err := json.Marshal(cleaned)
	if err != nil {
		return nil, false, err
	}
	result := string(encoded)
	return &result, true, nil
}

// ReferenceURLsOnly drops uploaded data URIs after a task becomes terminal.
// Ordinary upstream HTTP(S) references remain available as lightweight text.
func ReferenceURLsOnly(raw string) (*string, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, false
	}
	var items []string
	if strings.HasPrefix(text, "[") && json.Unmarshal([]byte(text), &items) == nil {
		kept := make([]string, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			prefix := ""
			value := item
			if strings.HasPrefix(value, "mask:") {
				prefix = "mask:"
				value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
			}
			if isHTTPURL(value) {
				kept = append(kept, prefix+value)
			}
		}
		if len(kept) == 0 {
			return nil, true
		}
		encoded, _ := json.Marshal(kept)
		result := string(encoded)
		return &result, result != text
	}
	prefix := ""
	value := text
	if strings.HasPrefix(value, "mask:") {
		prefix = "mask:"
		value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
	}
	if !isHTTPURL(value) {
		return nil, true
	}
	result := prefix + value
	return &result, result != text
}

func isInlineImage(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "data:image/")
}

func isHTTPURL(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://")
}
