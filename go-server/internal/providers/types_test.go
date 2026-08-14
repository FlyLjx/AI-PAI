package providers

import "testing"

func TestEffectiveBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		want     string
	}{
		{
			name:     "public by default",
			provider: Provider{BaseURL: " https://public.example.test/v1/ ", InternalBaseURL: "http://image-pool:8080/v1", UseInternalURL: false},
			want:     "https://public.example.test/v1",
		},
		{
			name:     "internal when enabled",
			provider: Provider{BaseURL: "https://public.example.test/v1", InternalBaseURL: " http://image-pool:8080/v1/ ", UseInternalURL: true},
			want:     "http://image-pool:8080/v1",
		},
		{
			name:     "public fallback for empty internal URL",
			provider: Provider{BaseURL: "https://public.example.test/v1/", UseInternalURL: true},
			want:     "https://public.example.test/v1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.provider.EffectiveBaseURL(); got != test.want {
				t.Fatalf("EffectiveBaseURL() = %q, want %q", got, test.want)
			}
		})
	}
}
