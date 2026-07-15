package httpserver

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestNormalizeAccountEmail(t *testing.T) {
	email, err := normalizeAccountEmail("  New.User+tag@Example.COM ")
	if err != nil {
		t.Fatal(err)
	}
	if email != "new.user+tag@example.com" {
		t.Fatalf("unexpected normalized email: %q", email)
	}
}

func TestNormalizeAccountEmailRejectsInvalidValues(t *testing.T) {
	values := []string{"", "not-an-email", "Name <user@example.com>", strings.Repeat("a", 111) + "@example.com"}
	for _, value := range values {
		if _, err := normalizeAccountEmail(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestEmailFromChangeToken(t *testing.T) {
	email := "user@example.com"
	token := base64.RawURLEncoding.EncodeToString([]byte(email)) + ".TOKEN"
	parsed, err := emailFromChangeToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != email {
		t.Fatalf("unexpected email: %q", parsed)
	}
}

func TestEmailFromChangeTokenRejectsMalformedOrNonCanonicalEmail(t *testing.T) {
	values := []string{
		"",
		"missing-dot",
		"invalid.TOKEN",
		base64.RawURLEncoding.EncodeToString([]byte("User@Example.com")) + ".TOKEN",
	}
	for _, value := range values {
		if _, err := emailFromChangeToken(value); err == nil {
			t.Fatalf("expected token %q to be rejected", value)
		}
	}
}

func TestEmailChangeTokenHashCoversEmbeddedEmail(t *testing.T) {
	first := base64.RawURLEncoding.EncodeToString([]byte("first@example.com")) + ".TOKEN"
	second := base64.RawURLEncoding.EncodeToString([]byte("second@example.com")) + ".TOKEN"
	if hashUserEmailToken(first) == hashUserEmailToken(second) {
		t.Fatal("changing the embedded email must change the stored token hash")
	}
}
