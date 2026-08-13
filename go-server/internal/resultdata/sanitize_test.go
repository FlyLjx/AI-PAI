package resultdata

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeResultJSONKeepsURLsAndUsage(t *testing.T) {
	raw := `{"data":[{"url":"https://example.test/image.png","b64_json":"IMAGE_BYTES"},{"base64":"MORE_BYTES"}],"usage":{"total_tokens":12}}`
	cleaned, changed, err := SanitizeResultJSON(raw)
	if err != nil || !changed || cleaned == nil {
		t.Fatalf("sanitize result: cleaned=%v changed=%v err=%v", cleaned, changed, err)
	}
	if strings.Contains(*cleaned, "IMAGE_BYTES") || strings.Contains(*cleaned, "base64") {
		t.Fatalf("inline image remained: %s", *cleaned)
	}
	if !strings.Contains(*cleaned, "https://example.test/image.png") || !strings.Contains(*cleaned, "total_tokens") {
		t.Fatalf("lightweight fields were removed: %s", *cleaned)
	}
}

func TestReferenceURLsOnlyRemovesUploadedImages(t *testing.T) {
	raw := `["data:image/png;base64,IMAGE_BYTES","https://example.test/reference.png","mask:data:image/png;base64,MASK"]`
	cleaned, changed := ReferenceURLsOnly(raw)
	if !changed || cleaned == nil {
		t.Fatalf("cleaned=%v changed=%v", cleaned, changed)
	}
	var values []string
	if err := json.Unmarshal([]byte(*cleaned), &values); err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0] != "https://example.test/reference.png" {
		t.Fatalf("unexpected references: %#v", values)
	}
}

func TestReferenceURLsOnlyClearsBase64OnlyValue(t *testing.T) {
	cleaned, changed := ReferenceURLsOnly("data:image/webp;base64,IMAGE_BYTES")
	if !changed || cleaned != nil {
		t.Fatalf("cleaned=%v changed=%v", cleaned, changed)
	}
}
