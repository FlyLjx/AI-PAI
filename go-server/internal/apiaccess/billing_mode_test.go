package apiaccess

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNormalizeNewBillingMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty defaults to balance", input: "", want: BillingModeBalance},
		{name: "whitespace defaults to balance", input: "   ", want: BillingModeBalance},
		{name: "subscription is accepted", input: BillingModeSubscription, want: BillingModeSubscription},
		{name: "balance is accepted", input: BillingModeBalance, want: BillingModeBalance},
		{name: "mode is normalized", input: " Subscription ", want: BillingModeSubscription},
		{name: "auto is rejected for new keys", input: BillingModeAuto, wantErr: true},
		{name: "unknown mode is rejected", input: "credits", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeNewBillingMode(test.input)
			if test.wantErr {
				if !errors.Is(err, ErrInvalidBillingMode) {
					t.Fatalf("error = %v, want ErrInvalidBillingMode", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("mode = %q, want %q", got, test.want)
			}
		})
	}
}

func TestToPublicKeyNormalizesStoredBillingMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want string
	}{
		{name: "empty historical mode becomes auto", mode: "", want: BillingModeAuto},
		{name: "subscription remains subscription", mode: BillingModeSubscription, want: BillingModeSubscription},
		{name: "balance remains balance", mode: BillingModeBalance, want: BillingModeBalance},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			public := ToPublicKey(AccessKey{BillingMode: test.mode})
			if public.BillingMode != test.want {
				t.Fatalf("billingMode = %q, want %q", public.BillingMode, test.want)
			}
		})
	}
}

func TestScanAccessKeysNormalizesHistoricalBillingMode(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	columns := []string{
		"id", "user_id", "user_email", "name", "key_prefix", "key_hash", "key_plain",
		"status", "concurrency_limit", "billing_mode", "last_used_at", "deleted_at",
		"created_at", "updated_at", "request_count", "success_count", "failed_count",
		"image_count", "last_error",
	}
	mock.ExpectQuery(`SELECT billing mode fixtures`).WillReturnRows(
		sqlmock.NewRows(columns).
			AddRow("key-auto", "user-1", "one@example.com", "Legacy", "sk-legacy", "hash-1", nil, "active", 10, nil, nil, nil, now, now, 0, 0, 0, 0, nil).
			AddRow("key-subscription", "user-1", "one@example.com", "Subscription", "sk-sub", "hash-2", nil, "active", 10, BillingModeSubscription, nil, nil, now, now, 0, 0, 0, 0, nil).
			AddRow("key-balance", "user-1", "one@example.com", "Balance", "sk-balance", "hash-3", nil, "active", 10, BillingModeBalance, nil, nil, now, now, 0, 0, 0, 0, nil),
	)

	rows, err := rawDB.Query("SELECT billing mode fixtures")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	items, err := scanAccessKeys(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	if items[0].BillingMode != BillingModeAuto {
		t.Fatalf("historical NULL mode = %q, want %q", items[0].BillingMode, BillingModeAuto)
	}
	if items[1].BillingMode != BillingModeSubscription {
		t.Fatalf("subscription mode = %q, want %q", items[1].BillingMode, BillingModeSubscription)
	}
	if items[2].BillingMode != BillingModeBalance {
		t.Fatalf("balance mode = %q, want %q", items[2].BillingMode, BillingModeBalance)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
