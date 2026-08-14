package generation

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"net/url"
	"strconv"
	"strings"

	"aipi-go/internal/providers"
	"aipi-go/internal/resultdata"
)

type ExtractedImage struct {
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
	B64  string `json:"b64_json,omitempty"`
}

func NormalizeImageResult(value any) any {
	images := uniqueImages(ExtractImages(value))
	return normalizedImageResult(images, ExtractImageUsage(value))
}

func NormalizeImageResultForProvider(value any, provider providers.Provider) any {
	images := uniqueImages(ExtractImages(rewriteUpstreamResultURLs(value, provider, 0)))
	return normalizedImageResult(images, ExtractImageUsage(value))
}

func normalizedImageResult(images []ExtractedImage, usage map[string]int) map[string]any {
	result := map[string]any{"data": images}
	if len(usage) > 0 {
		result["usage"] = usage
	}
	return result
}

// ResultWithoutBase64 preserves response metadata and URLs while recursively
// removing inline image bytes before persistence.
func ResultWithoutBase64(value any) any {
	if images, ok := value.([]ExtractedImage); ok {
		generic := make([]any, 0, len(images))
		for _, image := range images {
			if strings.TrimSpace(image.URL) != "" {
				generic = append(generic, map[string]any{"type": "url", "url": image.URL})
			}
		}
		if len(generic) == 0 {
			return nil
		}
		return resultdata.WithoutInlineImages(generic)
	}
	return resultdata.WithoutInlineImages(value)
}

// ExtractImageUsage normalizes image and chat token names without inventing
// values when the upstream response omits usage or individual fields.
func ExtractImageUsage(value any) map[string]int {
	payload, ok := stringAnyMap(value)
	if !ok {
		return nil
	}
	usage, ok := stringAnyMap(payload["usage"])
	if !ok {
		return nil
	}
	result := map[string]int{}
	copyUsageField(result, usage, "input_tokens", "input_tokens", "prompt_tokens", "inputTokens", "promptTokens")
	copyUsageField(result, usage, "output_tokens", "output_tokens", "completion_tokens", "outputTokens", "completionTokens")
	copyUsageField(result, usage, "total_tokens", "total_tokens", "totalTokens")
	if len(result) == 0 {
		return nil
	}
	return result
}

func AddImageUsage(total map[string]int, value any) map[string]int {
	usage := ExtractImageUsage(value)
	if len(usage) == 0 {
		return total
	}
	if total == nil {
		total = map[string]int{}
	}
	for key, count := range usage {
		total[key] += count
	}
	return total
}

func copyUsageField(result map[string]int, usage map[string]any, outputKey string, aliases ...string) {
	for _, key := range aliases {
		if count, ok := imageTokenCount(usage[key]); ok {
			result[outputKey] = count
			return
		}
	}
}

func stringAnyMap(value any) (map[string]any, bool) {
	switch payload := value.(type) {
	case map[string]any:
		return payload, true
	case map[string]int:
		result := make(map[string]any, len(payload))
		for key, item := range payload {
			result[key] = item
		}
		return result, true
	case map[string]float64:
		result := make(map[string]any, len(payload))
		for key, item := range payload {
			result[key] = item
		}
		return result, true
	default:
		return nil, false
	}
}

func imageTokenCount(value any) (int, bool) {
	var count int64
	switch number := value.(type) {
	case int:
		if number < 0 {
			return 0, false
		}
		return number, true
	case int32:
		count = int64(number)
	case int64:
		count = number
	case uint:
		if uint64(number) > uint64(math.MaxInt) {
			return 0, false
		}
		return int(number), true
	case uint64:
		if number > uint64(math.MaxInt) {
			return 0, false
		}
		return int(number), true
	case float32:
		return imageFloatTokenCount(float64(number))
	case float64:
		return imageFloatTokenCount(number)
	case json.Number:
		parsed, err := number.Int64()
		if err != nil {
			return 0, false
		}
		count = parsed
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(number), 10, 64)
		if err != nil {
			return 0, false
		}
		count = parsed
	default:
		return 0, false
	}
	if count < 0 || uint64(count) > uint64(math.MaxInt) {
		return 0, false
	}
	return int(count), true
}

func imageFloatTokenCount(value float64) (int, bool) {
	if value < 0 || value > float64(math.MaxInt) || math.Trunc(value) != value {
		return 0, false
	}
	return int(value), true
}

func ExtractImages(value any) []ExtractedImage {
	return extractImages(value, 0)
}

func extractImages(value any, depth int) []ExtractedImage {
	if value == nil || depth > 10 {
		return nil
	}
	if text, ok := value.(string); ok {
		if isImageURL(text) {
			return []ExtractedImage{{Type: "url", URL: text}}
		}
		if image, ok := extractBase64Image(text, false); ok {
			return []ExtractedImage{image}
		}
		return nil
	}
	if images, ok := value.([]ExtractedImage); ok {
		return images
	}
	if list, ok := value.([]any); ok {
		result := []ExtractedImage{}
		for _, item := range list {
			result = append(result, extractImages(item, depth+1)...)
		}
		return result
	}
	payload, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	result := []ExtractedImage{}
	for _, key := range []string{"url", "image_url", "imageUrl", "output_url", "outputUrl", "file_url", "fileUrl"} {
		if text, ok := payload[key].(string); ok && isImageURL(text) {
			result = append(result, ExtractedImage{Type: "url", URL: text})
		}
	}
	for _, key := range []string{"b64_json", "base64"} {
		if text, ok := payload[key].(string); ok {
			if image, ok := extractBase64Image(text, true); ok {
				result = append(result, image)
			}
		}
	}
	for _, key := range []string{"data", "result", "results", "output", "outputs", "images", "image", "final", "choices", "message", "content"} {
		result = append(result, extractImages(payload[key], depth+1)...)
	}
	return result
}

func uniqueImages(images []ExtractedImage) []ExtractedImage {
	seen := map[string]bool{}
	result := []ExtractedImage{}
	for _, image := range images {
		key := image.Type + ":" + image.URL + ":" + image.B64
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, image)
	}
	return result
}

func isImageURL(value string) bool {
	return len(value) > 4 && (hasPrefix(value, "http://") || hasPrefix(value, "https://"))
}

func extractBase64Image(value string, allowRaw bool) (ExtractedImage, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ExtractedImage{}, false
	}
	if image, ok := base64ImageFromDataURL(trimmed); ok {
		return image, true
	}
	if !allowRaw {
		return ExtractedImage{}, false
	}
	encoded := compactBase64(trimmed)
	if len(encoded) < 24 || strings.Contains(encoded, "://") || strings.Contains(encoded, ",") {
		return ExtractedImage{}, false
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) == 0 {
		return ExtractedImage{}, false
	}
	if !strings.HasPrefix(imageContentType("", decoded), "image/") {
		return ExtractedImage{}, false
	}
	return ExtractedImage{Type: "b64_json", B64: encoded}, true
}

func base64ImageFromDataURL(value string) (ExtractedImage, bool) {
	header, payload, ok := strings.Cut(strings.TrimSpace(value), ",")
	if !ok {
		return ExtractedImage{}, false
	}
	header = strings.ToLower(strings.TrimSpace(header))
	if !strings.HasPrefix(header, "data:image/") || !strings.Contains(header, ";base64") {
		return ExtractedImage{}, false
	}
	encoded := compactBase64(payload)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) == 0 {
		return ExtractedImage{}, false
	}
	return ExtractedImage{Type: "b64_json", B64: encoded}, true
}

func rewriteUpstreamResultURLs(value any, provider providers.Provider, depth int) any {
	if value == nil || depth > 10 {
		return value
	}
	if text, ok := value.(string); ok {
		return rewriteUpstreamImageURL(provider, text)
	}
	if images, ok := value.([]ExtractedImage); ok {
		result := make([]ExtractedImage, 0, len(images))
		for _, image := range images {
			image.URL = rewriteUpstreamImageURL(provider, image.URL)
			result = append(result, image)
		}
		return result
	}
	if list, ok := value.([]any); ok {
		result := make([]any, 0, len(list))
		for _, item := range list {
			result = append(result, rewriteUpstreamResultURLs(item, provider, depth+1))
		}
		return result
	}
	payload, ok := value.(map[string]any)
	if !ok {
		return value
	}
	result := map[string]any{}
	for key, item := range payload {
		result[key] = rewriteUpstreamResultURLs(item, provider, depth+1)
	}
	return result
}

func rewriteUpstreamImageURL(provider providers.Provider, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.HasPrefix(trimmed, "data:image/") {
		return value
	}
	if _, ok := extractBase64Image(trimmed, true); ok {
		return value
	}
	providerURL, err := url.Parse(strings.TrimSpace(provider.BaseURL))
	if err != nil || providerURL.Scheme == "" || providerURL.Host == "" {
		return value
	}
	if strings.HasPrefix(trimmed, "/") {
		return providerURL.Scheme + "://" + providerURL.Host + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return value
	}
	if internalURL, err := url.Parse(strings.TrimSpace(provider.InternalBaseURL)); err == nil &&
		internalURL.Hostname() != "" &&
		strings.EqualFold(parsed.Hostname(), internalURL.Hostname()) {
		parsed.Scheme = providerURL.Scheme
		parsed.Host = providerURL.Host
		return parsed.String()
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "127.0.0.1", "localhost", "::1", "0.0.0.0":
		parsed.Scheme = providerURL.Scheme
		parsed.Host = providerURL.Host
		return parsed.String()
	default:
		return value
	}
}

func hasPrefix(value string, prefix string) bool {
	if len(value) < len(prefix) {
		return false
	}
	return value[:len(prefix)] == prefix
}
