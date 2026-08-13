package operations

import (
	"testing"
)

func TestGenerationUsageQuantityUsesActualSuccessImages(t *testing.T) {
	resultJSON := `{"data":[{"url":"https://cdn.example.test/a.png"},{"url":"https://cdn.example.test/b.png"},{"url":"https://cdn.example.test/c.png"}]}`

	got := generationUsageQuantity("success", 5, resultJSON)
	if got != 3 {
		t.Fatalf("expected actual result image count, got %d", got)
	}
}

func TestGenerationUsageQuantityReservesRequestedQuantityForRunningTask(t *testing.T) {
	got := generationUsageQuantity("processing", 5, "")
	if got != 5 {
		t.Fatalf("expected requested quantity reservation, got %d", got)
	}
}

func TestGenerationUsageQuantityFallsBackToRequestedQuantity(t *testing.T) {
	got := generationUsageQuantity("success", 2, `{"data":[]}`)
	if got != 2 {
		t.Fatalf("expected fallback quantity, got %d", got)
	}
}
