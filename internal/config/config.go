package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the hotpod server.
type Config struct {
	// Port is the HTTP server port (default: 8080)
	Port int
	// LogLevel is the slog level: debug, info, warn, error (default: info)
	LogLevel string
	// StartupDelay is the time to wait before becoming ready
	StartupDelay time.Duration
	// StartupJitter adds random variance to StartupDelay
	StartupJitter time.Duration
	// ShutdownDelay is the pre-stop delay after receiving SIGTERM
	ShutdownDelay time.Duration
	// ShutdownTimeout is the max time to wait for in-flight requests
	ShutdownTimeout time.Duration
	// DrainImmediately rejects new requests immediately on shutdown
	DrainImmediately bool
	// RequestTimeout is the server-side timeout for all requests
	RequestTimeout time.Duration
	// MaxConcurrentOps is the max concurrent operations per type (<=0 to disable)
	MaxConcurrentOps int
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Port:             8080,
		LogLevel:         "info",
		ShutdownTimeout:  30 * time.Second,
		RequestTimeout:   5 * time.Minute,
		MaxConcurrentOps: 100,
	}

	var err error

	if cfg.Port, err = getEnvInt("HOTPOD_PORT", cfg.Port); err != nil {
		return nil, err
	}
	cfg.LogLevel = getEnvString("HOTPOD_LOG_LEVEL", cfg.LogLevel)
	if cfg.StartupDelay, err = getEnvDuration("HOTPOD_STARTUP_DELAY", cfg.StartupDelay); err != nil {
		return nil, err
	}
	if cfg.StartupJitter, err = getEnvDuration("HOTPOD_STARTUP_JITTER", cfg.StartupJitter); err != nil {
		return nil, err
	}
	if cfg.ShutdownDelay, err = getEnvDuration("HOTPOD_SHUTDOWN_DELAY", cfg.ShutdownDelay); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = getEnvDuration("HOTPOD_SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		return nil, err
	}
	if cfg.DrainImmediately, err = getEnvBool("HOTPOD_DRAIN_IMMEDIATELY", cfg.DrainImmediately); err != nil {
		return nil, err
	}
	if cfg.RequestTimeout, err = getEnvDuration("HOTPOD_REQUEST_TIMEOUT", cfg.RequestTimeout); err != nil {
		return nil, err
	}
	if cfg.MaxConcurrentOps, err = getEnvInt("HOTPOD_MAX_CONCURRENT_OPS", cfg.MaxConcurrentOps); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getEnvString(key, defaultVal string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal, nil
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return i, nil
}

func getEnvDuration(key string, defaultVal time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return d, nil
}

func getEnvBool(key string, defaultVal bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid %s: %w", key, err)
	}
	return b, nil
}

// Validate checks that configuration values are valid.
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}

	if c.StartupDelay < 0 {
		return fmt.Errorf("startup delay must be non-negative, got %s", c.StartupDelay)
	}

	if c.StartupJitter < 0 {
		return fmt.Errorf("startup jitter must be non-negative, got %s", c.StartupJitter)
	}

	if c.ShutdownDelay < 0 {
		return fmt.Errorf("shutdown delay must be non-negative, got %s", c.ShutdownDelay)
	}

	if c.ShutdownTimeout < 0 {
		return fmt.Errorf("shutdown timeout must be non-negative, got %s", c.ShutdownTimeout)
	}

	if c.RequestTimeout < 0 {
		return fmt.Errorf("request timeout must be non-negative, got %s", c.RequestTimeout)
	}

	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level %q, must be one of: debug, info, warn, error", c.LogLevel)
	}

	return nil
}
