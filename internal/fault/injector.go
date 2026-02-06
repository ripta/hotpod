package fault

import (
	"math/rand/v2"
	"sync"
	"time"
)

// ErrorConfig holds the error injection configuration for an endpoint.
type ErrorConfig struct {
	// Rate is the probability of injecting an error (0.0 to 1.0)
	Rate float64
	// Codes is the list of HTTP status codes to randomly select from
	Codes []int
	// ExpiresAt is when this configuration expires (zero means never)
	ExpiresAt time.Time
}

// IsExpired returns true if the configuration has expired.
func (c *ErrorConfig) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt)
}

// ShouldInject returns true if an error should be injected based on the rate.
func (c *ErrorConfig) ShouldInject() bool {
	if c.Rate <= 0 {
		return false
	}
	if c.Rate >= 1 {
		return true
	}
	return rand.Float64() < c.Rate
}

// SelectCode returns a random status code from the configured codes.
func (c *ErrorConfig) SelectCode() int {
	if len(c.Codes) == 0 {
		return 500
	}
	if len(c.Codes) == 1 {
		return c.Codes[0]
	}
	return c.Codes[rand.IntN(len(c.Codes))]
}

// Injector manages error injection configuration for endpoints.
type Injector struct {
	mu sync.RWMutex
	// configs maps endpoint paths to their error configuration
	configs map[string]*ErrorConfig
	// globalConfig applies to all endpoints if set
	globalConfig *ErrorConfig
}

// NewInjector creates a new error injector.
func NewInjector() *Injector {
	return &Injector{
		configs: make(map[string]*ErrorConfig),
	}
}

// SetEndpointConfig sets the error configuration for a specific endpoint.
func (i *Injector) SetEndpointConfig(endpoint string, cfg *ErrorConfig) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if cfg == nil || cfg.Rate <= 0 {
		delete(i.configs, endpoint)
	} else {
		i.configs[endpoint] = cfg
	}
}

// SetGlobalConfig sets the global error configuration that applies to all endpoints.
func (i *Injector) SetGlobalConfig(cfg *ErrorConfig) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.globalConfig = cfg
}

// GetConfig returns the error configuration for an endpoint.
// Returns the endpoint-specific config if set, otherwise the global config.
// Returns nil if no config applies.
func (i *Injector) GetConfig(endpoint string) *ErrorConfig {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if cfg, ok := i.configs[endpoint]; ok {
		if !cfg.IsExpired() {
			return cfg
		}
	}

	if i.globalConfig != nil && !i.globalConfig.IsExpired() {
		return i.globalConfig
	}

	return nil
}

// ShouldInjectError checks if an error should be injected for the given endpoint.
// Returns the status code to inject, or 0 if no error should be injected.
func (i *Injector) ShouldInjectError(endpoint string) int {
	cfg := i.GetConfig(endpoint)
	if cfg == nil {
		return 0
	}
	if !cfg.ShouldInject() {
		return 0
	}
	return cfg.SelectCode()
}

// Reset clears all error injection configuration.
func (i *Injector) Reset() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.configs = make(map[string]*ErrorConfig)
	i.globalConfig = nil
}

// GetGlobalConfig returns the current global error configuration, or nil if not set.
func (i *Injector) GetGlobalConfig() *ErrorConfig {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.globalConfig != nil && !i.globalConfig.IsExpired() {
		return i.globalConfig
	}
	return nil
}

// GetEndpointConfigs returns a copy of all endpoint-specific error configurations.
func (i *Injector) GetEndpointConfigs() map[string]*ErrorConfig {
	i.mu.RLock()
	defer i.mu.RUnlock()
	result := make(map[string]*ErrorConfig, len(i.configs))
	for k, v := range i.configs {
		if !v.IsExpired() {
			result[k] = v
		}
	}
	return result
}

// GetEndpointRate returns the current error rate for an endpoint (for metrics).
func (i *Injector) GetEndpointRate(endpoint string) float64 {
	cfg := i.GetConfig(endpoint)
	if cfg == nil {
		return 0
	}
	return cfg.Rate
}
