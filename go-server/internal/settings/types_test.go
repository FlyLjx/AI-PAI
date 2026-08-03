package settings

import (
	"testing"
	"time"
)

func TestSupportGroupSettingsArePublic(t *testing.T) {
	values := Settings{
		"supportGroupNumber": "123456",
		"supportGroupUrl":    "https://example.com/group",
	}
	public := Public(values)
	for _, key := range []string{"supportGroupNumber", "supportGroupUrl"} {
		if _, ok := Defaults[key]; !ok {
			t.Fatalf("Defaults missing %s", key)
		}
		if public[key] != values[key] {
			t.Fatalf("Public(%s) = %v, want %v", key, public[key], values[key])
		}
	}
}

func TestRechargeSettingsRemainAvailable(t *testing.T) {
	for _, key := range []string{"creditName", "rechargeEnabled", "rechargeRate", "rechargeMinAmount", "rechargePresets"} {
		value, ok := Defaults[key]
		if !ok {
			t.Fatalf("Defaults missing %s", key)
		}
		if Public(Defaults)[key] != value {
			t.Fatalf("Public settings missing %s", key)
		}
	}
	if Defaults["rechargeRate"] != float64(10) {
		t.Fatalf("default recharge rate = %v, want 10", Defaults["rechargeRate"])
	}
}

func TestDynamicConcurrencyDefaults(t *testing.T) {
	want := Settings{
		"dynamicConcurrencyEnabled":     true,
		"dynamicConcurrencyWindowValue": float64(1),
		"dynamicConcurrencyWindowUnit":  "hour",
		"dynamicConcurrencyRequestStep": float64(50),
		"dynamicConcurrencyIncrement":   float64(5),
	}
	for key, value := range want {
		if Defaults[key] != value {
			t.Fatalf("Defaults[%s] = %v, want %v", key, Defaults[key], value)
		}
	}
}

func TestSystemLogCleanupDefaults(t *testing.T) {
	if Defaults["systemLogAutoCleanupEnabled"] != false {
		t.Fatalf("default system log cleanup enabled = %v, want false", Defaults["systemLogAutoCleanupEnabled"])
	}
	if Defaults["systemLogRetentionDays"] != float64(30) {
		t.Fatalf("default system log retention days = %v, want 30", Defaults["systemLogRetentionDays"])
	}
	if _, ok := Public(Defaults)["systemLogRetentionDays"]; ok {
		t.Fatal("system log retention must remain admin-only")
	}
}

func TestTaskImageCleanupDefaults(t *testing.T) {
	if Defaults["taskImageAutoCleanupEnabled"] != true {
		t.Fatalf("default task image cleanup enabled = %v, want true", Defaults["taskImageAutoCleanupEnabled"])
	}
	if Defaults["taskImageRetentionDays"] != float64(1) {
		t.Fatalf("default task image retention days = %v, want 1", Defaults["taskImageRetentionDays"])
	}
	if _, ok := Public(Defaults)["taskImageRetentionDays"]; ok {
		t.Fatal("task image retention must remain admin-only")
	}
}

func TestTaskTimeout(t *testing.T) {
	if got := TaskTimeout(Settings{"taskTimeoutMinutes": float64(10)}); got != 10*time.Minute {
		t.Fatalf("task timeout = %s, want 10m", got)
	}
	if got := TaskTimeout(Settings{"taskTimeoutMinutes": float64(0)}); got != DefaultTaskTimeoutMinutes*time.Minute {
		t.Fatalf("invalid task timeout = %s, want default 5m", got)
	}
}
