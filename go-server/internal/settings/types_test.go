package settings

import "testing"

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
