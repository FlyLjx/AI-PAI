package httpserver

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aipi-go/internal/models"
	"aipi-go/internal/operations"
)

func TestReferenceImagePayloadAbsolutizesRelativeURLs(t *testing.T) {
	req := httptest.NewRequest("POST", "http://example.test/api/generate/image", nil)
	payload := referenceImagePayload(req, generateImageInput{
		ReferenceImageURL:  "/api/tasks/task-1/images/0",
		ReferenceImageURLs: []string{"/api/tasks/task-1/images/0", "data:image/png;base64,abc"},
		MaskImageURL:       "/api/tasks/task-1/images/0/mask",
	})
	if payload == nil {
		t.Fatal("expected reference payload")
	}
	var items []string
	if err := json.Unmarshal([]byte(*payload), &items); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	want := []string{
		"http://example.test/api/tasks/task-1/images/0",
		"data:image/png;base64,abc",
		"mask:http://example.test/api/tasks/task-1/images/0/mask",
	}
	if len(items) != len(want) {
		t.Fatalf("expected %d payload items, got %#v", len(want), items)
	}
	for index := range want {
		if items[index] != want[index] {
			t.Fatalf("payload item %d should be %q, got %q", index, want[index], items[index])
		}
	}
}

func TestCompatEditReferencePayloadAbsolutizesRelativeURLs(t *testing.T) {
	req := httptest.NewRequest("POST", "https://aipi.example.test/v1/images/edits", nil)
	payload := compatEditReferencePayload(req, compatImageInput{
		ImageURL: map[string]any{"url": "/api/tasks/task-2/images/0"},
		Mask:     "/api/tasks/task-2/images/0/mask",
	})
	if payload == nil {
		t.Fatal("expected reference payload")
	}
	var items []string
	if err := json.Unmarshal([]byte(*payload), &items); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	want := []string{
		"https://aipi.example.test/api/tasks/task-2/images/0",
		"mask:https://aipi.example.test/api/tasks/task-2/images/0/mask",
	}
	if len(items) != len(want) {
		t.Fatalf("expected %d payload items, got %#v", len(want), items)
	}
	for index := range want {
		if items[index] != want[index] {
			t.Fatalf("payload item %d should be %q, got %q", index, want[index], items[index])
		}
	}
}

func TestCompatEditReferencePayloadSupportsOpenAIEditAliases(t *testing.T) {
	req := httptest.NewRequest("POST", "https://aipi.example.test/v1/images/edits", nil)
	payload := compatEditReferencePayload(req, compatImageInput{
		Images:          []string{"/api/tasks/task-3/images/0"},
		ImageURLs:       []any{map[string]any{"url": "/api/tasks/task-3/images/1"}},
		ReferenceItems:  []any{"/api/tasks/task-3/images/2"},
		ReferenceImages: []any{map[string]any{"image_url": map[string]any{"url": "/api/tasks/task-3/images/3"}}},
		InputImages:     []any{"/api/tasks/task-3/images/4"},
	})
	if payload == nil {
		t.Fatal("expected reference payload")
	}
	var items []string
	if err := json.Unmarshal([]byte(*payload), &items); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	want := []string{
		"https://aipi.example.test/api/tasks/task-3/images/1",
		"https://aipi.example.test/api/tasks/task-3/images/0",
		"https://aipi.example.test/api/tasks/task-3/images/2",
		"https://aipi.example.test/api/tasks/task-3/images/3",
		"https://aipi.example.test/api/tasks/task-3/images/4",
	}
	if len(items) != len(want) {
		t.Fatalf("expected %d payload items, got %#v", len(want), items)
	}
	for index := range want {
		if items[index] != want[index] {
			t.Fatalf("payload item %d should be %q, got %q", index, want[index], items[index])
		}
	}
}

func TestDecodeCompatImageInputSupportsMultipartImageArrayFields(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image[]", "ref.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(testPNGBytes(t)); err != nil {
		t.Fatal(err)
	}
	part, err = writer.CreateFormFile("image[1]", "ref-2.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(testPNGBytes(t)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("prompt", "edit prompt"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "https://aipi.example.test/v1/images/edits", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	var input compatImageInput
	if err := decodeCompatImageInput(req, &input, true); err != nil {
		t.Fatal(err)
	}
	if input.Prompt != "edit prompt" {
		t.Fatalf("prompt=%q", input.Prompt)
	}
	if len(input.ReferenceURLs) != 2 {
		t.Fatalf("reference count=%d, want 2", len(input.ReferenceURLs))
	}
	for _, item := range input.ReferenceURLs {
		if !strings.HasPrefix(item, "data:image/png;base64,") {
			t.Fatalf("reference should be png data URL, got %.40q", item)
		}
	}
}

func TestCompatRequestReferenceURLsLimitsUniqueImages(t *testing.T) {
	input := compatImageInput{ReferenceURLs: []string{"one", "two", "three", "four", "four"}}
	if count := len(compatRequestReferenceURLs(input, true)); count != 4 {
		t.Fatalf("unique reference count=%d, want 4", count)
	}
	input.ReferenceURLs = append(input.ReferenceURLs, "five")
	if count := len(compatRequestReferenceURLs(input, true)); count <= maxCompatReferenceImages {
		t.Fatalf("reference count=%d, want more than %d", count, maxCompatReferenceImages)
	}
}

func testPNGBytes(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestGenerationBalanceCostUsesDatabasePrecision(t *testing.T) {
	cost := generationBalanceCost(0.1, 3)
	if cost != 0.3 {
		t.Fatalf("cost = %v, want 0.3", cost)
	}
	if !hasAvailableGenerationBalance(1, 0.7, cost) {
		t.Fatal("expected exact remaining balance to cover the quote")
	}
	if hasAvailableGenerationBalance(1, 0.7001, 0.3) {
		t.Fatal("expected active reservations to prevent over-commit")
	}
}

func TestAbsoluteURLUsesForwardedOrigin(t *testing.T) {
	req := httptest.NewRequest("GET", "http://api:3001/v1/images/generations", nil)
	req.Header.Set("X-Forwarded-Proto", "https, http")
	req.Header.Set("X-Forwarded-Host", "app.example.test, api:3001")
	if got := absoluteURL(req, "/api/tasks/task-1/images/0"); got != "https://app.example.test/api/tasks/task-1/images/0" {
		t.Fatalf("absolute URL = %q", got)
	}
}

func TestPaidSubscriptionQuotaValidationUsesQuotedEntitlement(t *testing.T) {
	entitlement := &operations.SubscriptionEntitlement{
		IsPaid:             true,
		QuotaRemaining:     2,
		AllowedProviderIDs: []string{"provider-1"},
		AllowedModelIDs:    []string{"model-1"},
	}
	model := models.Model{ID: "model-1", ProviderID: "provider-1"}
	if err := requireGenerationQuotaForEntitlement(entitlement, model, 2); err != nil {
		t.Fatal(err)
	}
	if err := requireGenerationQuotaForEntitlement(entitlement, model, 3); err == nil {
		t.Fatal("expected exhausted subscription quota to reject the request")
	}
	model.ID = "model-2"
	if err := requireGenerationQuotaForEntitlement(entitlement, model, 1); err == nil {
		t.Fatal("expected subscription model restrictions to be enforced")
	}
}
