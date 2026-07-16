package users

import (
	"errors"
	"testing"
	"time"
)

func TestRequireEmailVerifiedForAPIKey(t *testing.T) {
	if err := RequireEmailVerifiedForAPIKey(&User{}); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("unverified user error = %v, want ErrEmailNotVerified", err)
	}

	verifiedAt := time.Now()
	if err := RequireEmailVerifiedForAPIKey(&User{EmailVerifiedAt: &verifiedAt}); err != nil {
		t.Fatalf("verified user error = %v, want nil", err)
	}
}
