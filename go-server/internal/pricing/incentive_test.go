package pricing

import "testing"

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
