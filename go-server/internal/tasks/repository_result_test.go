package tasks

import (
	"strings"
	"testing"
)

func TestDurableResultJSONRemovesInlineImageData(t *testing.T) {
	result := map[string]any{
		"data": []any{
			map[string]any{
				"type":     "b64_json",
				"b64_json": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB",
				"url":      "https://example.test/image.png",
			},
		},
		"usage": map[string]any{"total_tokens": 12},
	}

	encoded, err := durableResultJSON(result)
	if err != nil {
		t.Fatal(err)
	}
	if encoded == nil {
		t.Fatal("expected URL and usage metadata to remain")
	}
	text := encoded.(string)
	if strings.Contains(text, `"b64_json":"`) || strings.Contains(text, "iVBORw0KGgo") {
		t.Fatalf("durable result contains inline image data: %s", text)
	}
	if !strings.Contains(text, "https://example.test/image.png") || !strings.Contains(text, "total_tokens") {
		t.Fatalf("durable result dropped metadata: %s", text)
	}
}

func TestDurableResultJSONSanitizesStructValues(t *testing.T) {
	type image struct {
		B64 string `json:"b64_json,omitempty"`
		URL string `json:"url,omitempty"`
	}
	result := struct {
		Data []image `json:"data"`
	}{Data: []image{{B64: "IMAGE_BYTES", URL: "https://example.test/image.png"}}}

	encoded, err := durableResultJSON(result)
	if err != nil {
		t.Fatal(err)
	}
	if encoded == nil || strings.Contains(encoded.(string), "b64_json") || strings.Contains(encoded.(string), "IMAGE_BYTES") {
		t.Fatalf("struct result was not sanitized: %#v", encoded)
	}
}

func TestErrorDetailsResultJSONRemovesInlineImageData(t *testing.T) {
	encoded, err := errorDetailsResultJSON(map[string]any{
		"provider": "upstream",
		"b64_json": "IMAGE_BYTES",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, "b64_json") || strings.Contains(encoded, "IMAGE_BYTES") {
		t.Fatalf("error details contain inline image data: %s", encoded)
	}
	if !strings.Contains(encoded, errorDetailsResultKey) || !strings.Contains(encoded, "upstream") {
		t.Fatalf("error details lost metadata: %s", encoded)
	}
}
