package httpserver

import (
	"encoding/json"
	"testing"
)

func TestAlipayPrecreateOrderExpiresAfterThirtyMinutes(t *testing.T) {
	payload := map[string]string{}
	if err := json.Unmarshal([]byte(alipayPrecreateBizContent(alipaySettings{SiteName: "AI-PAI"}, "order-1", 19.9, "专业订阅")), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["timeout_express"] != "30m" {
		t.Fatalf("timeout_express = %q, want 30m", payload["timeout_express"])
	}
	if payload["out_trade_no"] != "order-1" || payload["total_amount"] != "19.90" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}
