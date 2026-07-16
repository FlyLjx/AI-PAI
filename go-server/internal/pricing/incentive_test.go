package pricing

import (
	"context"
	"testing"
)

func TestCurrentSubscriptionDiscountIsDisabled(t *testing.T) {
	discount, err := CurrentSubscriptionDiscount(context.Background(), nil, "user-with-legacy-discount")
	if err != nil {
		t.Fatal(err)
	}
	if discount != 0 {
		t.Fatalf("expected subscription discount to be disabled, got %v", discount)
	}
}

func TestApplyUnitPriceStillAppliesActivityDiscount(t *testing.T) {
	price, discount, source := ApplyUnitPrice(0.1, Result{
		Active:          true,
		DiscountPercent: 20,
		MinUnitPrice:    MinUnitPrice,
	}, 0)
	if price != 0.08 {
		t.Fatalf("expected activity price 0.08, got %v", price)
	}
	if discount != 20 {
		t.Fatalf("expected activity discount 20, got %v", discount)
	}
	if source != "activity" {
		t.Fatalf("expected activity discount source, got %q", source)
	}
}

func TestApplyUnitPriceAllowsFullSubscriptionDiscount(t *testing.T) {
	price, discount, source := ApplyUnitPrice(0.1, Result{}, 100)
	if price != 0 {
		t.Fatalf("expected free unit price for 100%% subscription discount, got %v", price)
	}
	if discount != 100 {
		t.Fatalf("expected applied discount 100, got %v", discount)
	}
	if source != "subscription" {
		t.Fatalf("expected subscription discount source, got %q", source)
	}
}

func TestApplyUnitPriceKeepsMinUnitForPartialDiscount(t *testing.T) {
	price, discount, source := ApplyUnitPrice(0.1, Result{}, 99)
	if price != MinUnitPrice {
		t.Fatalf("expected min unit price for partial discount, got %v", price)
	}
	if discount != 99 {
		t.Fatalf("expected applied discount 99, got %v", discount)
	}
	if source != "subscription" {
		t.Fatalf("expected subscription discount source, got %q", source)
	}
}
