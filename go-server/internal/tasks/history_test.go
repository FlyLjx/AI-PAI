package tasks

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestToHistoryOmitsImageAndResultData(t *testing.T) {
	referenceURL := "data:image/png;base64,REFERENCE"
	task := Task{
		ID:                "task-1",
		ModelID:           "model-1",
		ProviderID:        "provider-1",
		Prompt:            "draw a lighthouse",
		ReferenceImageURL: &referenceURL,
		Quantity:          1,
		Status:            StatusSuccess,
		ResultJSON:        map[string]any{"b64_json": "RESULT"},
		StoredResultURLs:  []string{"https://example.test/result.png"},
		CreatedAt:         time.Unix(1, 0).UTC(),
		UpdatedAt:         time.Unix(2, 0).UTC(),
	}

	payload, err := json.Marshal(ToHistory(&task))
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(payload)
	for _, forbidden := range []string{
		"referenceImageUrl", "resultJson", "resultUrl", "resultUrls",
		"directResultUrl", "directResultUrls", "thumbnailUrl", "thumbnailUrls",
		"REFERENCE", "RESULT", "result.png",
	} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("history response contains %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(encoded, `"prompt":"draw a lighthouse"`) {
		t.Fatalf("history response omitted key task information: %s", encoded)
	}
}
