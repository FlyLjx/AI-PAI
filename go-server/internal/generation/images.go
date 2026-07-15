package generation

import (
	"net/url"
	"strings"

	"aipi-go/internal/providers"
)

type ExtractedImage struct {
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
	B64  string `json:"b64_json,omitempty"`
}

func NormalizeImageResult(value any) any {
	images := uniqueImages(ExtractImages(value))
	return map[string]any{"data": images}
}

func NormalizeImageResultForProvider(value any, provider providers.Provider) any {
	images := uniqueImages(ExtractImages(rewriteUpstreamResultURLs(value, provider, 0)))
	return map[string]any{"data": images}
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
