package httpserver

import (
	"net/http/httptest"
	"testing"
)

func TestNormalizedRegistrationIP(t *testing.T) {
	for input, want := range map[string]string{
		"203.0.113.8:443":        "203.0.113.8",
		"[2001:db8::1]:8443":     "2001:db8::1",
		"198.51.100.2, 10.0.0.1": "198.51.100.2",
	} {
		if got := normalizedRegistrationIP(input); got != want {
			t.Fatalf("normalizedRegistrationIP(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRequestIPPrefersAndNormalizesForwardedAddress(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.test", nil)
	req.RemoteAddr = "192.0.2.10:43120"
	req.Header.Set("X-Forwarded-For", "[2001:db8::8]:443, 10.0.0.1")
	if got := requestIP(req); got != "2001:db8::8" {
		t.Fatalf("requestIP() = %q, want %q", got, "2001:db8::8")
	}
}

func TestRequestIPFallsBackWhenForwardedAddressIsInvalid(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.test", nil)
	req.RemoteAddr = "192.0.2.10:43120"
	req.Header.Set("X-Forwarded-For", "not-an-ip")
	if got := requestIP(req); got != "192.0.2.10" {
		t.Fatalf("requestIP() = %q, want %q", got, "192.0.2.10")
	}
}

func TestRegistrationFingerprintDoesNotHashMissingDevice(t *testing.T) {
	if got := hashOptionalRegistrationValue("device:"); got != "" {
		t.Fatalf("missing device hash = %q, want empty", got)
	}
	if got := hashOptionalRegistrationValue("device:abc"); len(got) != 64 {
		t.Fatalf("device hash length = %d, want 64", len(got))
	}
}
