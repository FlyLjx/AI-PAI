package httpserver

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"aipi-go/internal/models"
)

func TestCustomerPricingModelsOnlyExposeCallableImageModels(t *testing.T) {
	active := "active"
	disabled := "disabled"
	now := time.Now()
	items := []models.Model{
		{ID: "first", ModelName: "upstream-a", DisplayName: "Image Pro", Capability: "chat_image", Status: "active", ProviderStatus: &active, Price1K: 0.1, Price2K: 0.2, Price4K: 0.4, EnabledSizeTiers: []string{"1k", "2k"}, UpdatedAt: now},
		{ID: "duplicate", ModelName: "upstream-b", DisplayName: "Image Pro", Capability: "chat_image", Status: "active", ProviderStatus: &active, Price1K: 9, UpdatedAt: now},
		{ID: "disabled-provider", ModelName: "image-disabled-provider", DisplayName: "Provider Down", Capability: "chat_image", Status: "active", ProviderStatus: &disabled, UpdatedAt: now},
		{ID: "disabled-model", ModelName: "image-disabled", DisplayName: "Model Down", Capability: "chat_image", Status: "disabled", ProviderStatus: &active, UpdatedAt: now},
		{ID: "text", ModelName: "gpt-5.5", DisplayName: "GPT 5.5", Capability: "chat_image", Status: "active", ProviderStatus: &active, UpdatedAt: now},
		{ID: "other", ModelName: "chat-only", DisplayName: "Chat Only", Capability: "chat", Status: "active", ProviderStatus: &active, UpdatedAt: now},
	}

	result := customerPricingModels(items)
	if len(result) != 1 {
		t.Fatalf("pricing models = %d, want 1: %#v", len(result), result)
	}
	if result[0].ID != "Image Pro" || result[0].Price1K != 0.1 || len(result[0].EnabledSizeTiers) != 2 {
		t.Fatalf("unexpected public price model: %#v", result[0])
	}
	payload, err := json.Marshal(result[0])
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, hidden := range []string{"cost1k", "markupPercent", "providerId", "modelName"} {
		if strings.Contains(text, hidden) {
			t.Fatalf("public pricing payload exposes %s: %s", hidden, text)
		}
	}
}
