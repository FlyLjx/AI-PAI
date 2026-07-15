package providers

import "testing"

func TestAuthorizationHeaderNormalizesBearerPrefix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain key", input: "sk-test", want: "Bearer sk-test"},
		{name: "bearer key", input: "Bearer sk-test", want: "Bearer sk-test"},
		{name: "lowercase bearer", input: "bearer sk-test", want: "Bearer sk-test"},
		{name: "spaced bearer", input: "  Bearer   sk-test  ", want: "Bearer sk-test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AuthorizationHeader(tt.input); got != tt.want {
				t.Fatalf("AuthorizationHeader(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
