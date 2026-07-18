package httpserver

import "testing"

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

func TestRegistrationFingerprintDoesNotHashMissingDevice(t *testing.T) {
	if got := hashOptionalRegistrationValue("device:"); got != "" {
		t.Fatalf("missing device hash = %q, want empty", got)
	}
	if got := hashOptionalRegistrationValue("device:abc"); len(got) != 64 {
		t.Fatalf("device hash length = %d, want 64", len(got))
	}
}
