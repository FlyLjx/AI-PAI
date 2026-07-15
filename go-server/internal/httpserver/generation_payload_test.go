package httpserver

import (
	"encoding/json"
	"net/http/httptest"
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
