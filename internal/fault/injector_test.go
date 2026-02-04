package fault

import (
	"testing"
	"time"
)

func TestErrorConfigIsExpired(t *testing.T) {
	// Never expires
	cfg := &ErrorConfig{Rate: 0.5, Codes: []int{500}}
	if cfg.IsExpired() {
		t.Error("zero ExpiresAt should not be expired")
	}

	// Not yet expired
	cfg = &ErrorConfig{Rate: 0.5, Codes: []int{500}, ExpiresAt: time.Now().Add(time.Hour)}
	if cfg.IsExpired() {
		t.Error("future ExpiresAt should not be expired")
	}

	// Already expired
	cfg = &ErrorConfig{Rate: 0.5, Codes: []int{500}, ExpiresAt: time.Now().Add(-time.Hour)}
	if !cfg.IsExpired() {
		t.Error("past ExpiresAt should be expired")
	}
}

func TestErrorConfigShouldInject(t *testing.T) {
	// Zero rate never injects
	cfg := &ErrorConfig{Rate: 0, Codes: []int{500}}
	for range 10 {
		if cfg.ShouldInject() {
			t.Error("rate 0 should never inject")
		}
	}

	// Rate 1 always injects
	cfg = &ErrorConfig{Rate: 1, Codes: []int{500}}
	for range 10 {
		if !cfg.ShouldInject() {
			t.Error("rate 1 should always inject")
		}
	}

	// Rate 0.5 should inject some of the time
	cfg = &ErrorConfig{Rate: 0.5, Codes: []int{500}}
	injected := 0
	for range 100 {
		if cfg.ShouldInject() {
			injected++
		}
	}
	if injected == 0 {
		t.Error("rate 0.5 should inject sometimes")
	}
	if injected == 100 {
		t.Error("rate 0.5 should not always inject")
	}
}

func TestErrorConfigSelectCode(t *testing.T) {
	// Empty codes returns 500
	cfg := &ErrorConfig{Rate: 1, Codes: []int{}}
	if code := cfg.SelectCode(); code != 500 {
		t.Errorf("empty codes returned %d, want 500", code)
	}

	// Single code returns that code
	cfg = &ErrorConfig{Rate: 1, Codes: []int{503}}
	for range 10 {
		if code := cfg.SelectCode(); code != 503 {
			t.Errorf("single code returned %d, want 503", code)
		}
	}

	// Multiple codes returns one of them
	cfg = &ErrorConfig{Rate: 1, Codes: []int{500, 502, 503}}
	seen := make(map[int]bool)
	for range 100 {
		code := cfg.SelectCode()
		seen[code] = true
	}
	if len(seen) < 2 {
		t.Error("multiple codes should return variety of codes")
	}
	for code := range seen {
		if code != 500 && code != 502 && code != 503 {
			t.Errorf("unexpected code %d", code)
		}
	}
}

func TestInjectorSetEndpointConfig(t *testing.T) {
	inj := NewInjector()

	// Set config
	inj.SetEndpointConfig("/test", &ErrorConfig{Rate: 0.5, Codes: []int{500}})
	cfg := inj.GetConfig("/test")
	if cfg == nil {
		t.Fatal("expected config for /test")
	}
	if cfg.Rate != 0.5 {
		t.Errorf("rate = %f, want 0.5", cfg.Rate)
	}

	// Remove config with nil
	inj.SetEndpointConfig("/test", nil)
	if inj.GetConfig("/test") != nil {
		t.Error("expected nil after removing config")
	}

	// Remove config with zero rate
	inj.SetEndpointConfig("/test", &ErrorConfig{Rate: 0.5, Codes: []int{500}})
	inj.SetEndpointConfig("/test", &ErrorConfig{Rate: 0, Codes: []int{500}})
	if inj.GetConfig("/test") != nil {
		t.Error("expected nil after setting rate to 0")
	}
}

func TestInjectorGlobalConfig(t *testing.T) {
	inj := NewInjector()

	// No config returns nil
	if inj.GetConfig("/test") != nil {
		t.Error("expected nil with no config")
	}

	// Global config applies to all endpoints
	inj.SetGlobalConfig(&ErrorConfig{Rate: 0.5, Codes: []int{500}})
	cfg := inj.GetConfig("/test")
	if cfg == nil {
		t.Fatal("expected global config for /test")
	}

	// Endpoint config overrides global
	inj.SetEndpointConfig("/test", &ErrorConfig{Rate: 0.8, Codes: []int{503}})
	cfg = inj.GetConfig("/test")
	if cfg.Rate != 0.8 {
		t.Errorf("rate = %f, want 0.8 (endpoint should override global)", cfg.Rate)
	}

	// Other endpoints still use global
	cfg = inj.GetConfig("/other")
	if cfg.Rate != 0.5 {
		t.Errorf("rate = %f, want 0.5 (should use global)", cfg.Rate)
	}
}

func TestInjectorExpiredConfig(t *testing.T) {
	inj := NewInjector()

	// Set expired config
	inj.SetEndpointConfig("/test", &ErrorConfig{
		Rate:      0.5,
		Codes:     []int{500},
		ExpiresAt: time.Now().Add(-time.Hour),
	})

	// Expired config is not returned
	if inj.GetConfig("/test") != nil {
		t.Error("expired config should return nil")
	}

	// Set expired global config
	inj.SetGlobalConfig(&ErrorConfig{
		Rate:      0.5,
		Codes:     []int{500},
		ExpiresAt: time.Now().Add(-time.Hour),
	})
	if inj.GetConfig("/test") != nil {
		t.Error("expired global config should return nil")
	}
}

func TestInjectorShouldInjectError(t *testing.T) {
	inj := NewInjector()

	// No config returns 0
	if code := inj.ShouldInjectError("/test"); code != 0 {
		t.Errorf("no config returned %d, want 0", code)
	}

	// Always inject with rate 1
	inj.SetEndpointConfig("/test", &ErrorConfig{Rate: 1, Codes: []int{503}})
	if code := inj.ShouldInjectError("/test"); code != 503 {
		t.Errorf("rate 1 returned %d, want 503", code)
	}
}

func TestInjectorReset(t *testing.T) {
	inj := NewInjector()

	inj.SetEndpointConfig("/test", &ErrorConfig{Rate: 1, Codes: []int{500}})
	inj.SetGlobalConfig(&ErrorConfig{Rate: 0.5, Codes: []int{503}})

	inj.Reset()

	if inj.GetConfig("/test") != nil {
		t.Error("reset should clear endpoint config")
	}
	if inj.GetConfig("/other") != nil {
		t.Error("reset should clear global config")
	}
}

func TestInjectorGetEndpointRate(t *testing.T) {
	inj := NewInjector()

	// No config returns 0
	if rate := inj.GetEndpointRate("/test"); rate != 0 {
		t.Errorf("no config returned %f, want 0", rate)
	}

	// Returns configured rate
	inj.SetEndpointConfig("/test", &ErrorConfig{Rate: 0.75, Codes: []int{500}})
	if rate := inj.GetEndpointRate("/test"); rate != 0.75 {
		t.Errorf("rate = %f, want 0.75", rate)
	}
}
