package config

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// IOBasePath is the fixed base directory for I/O operations.
const IOBasePath = "/tmp"

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
	// MaxCPUDuration is the maximum duration for CPU load operations (default: 60s)
	MaxCPUDuration time.Duration
	// MaxMemorySize is the maximum memory allocation size in bytes (default: 1GB)
	MaxMemorySize int64
	// MaxIOSize is the maximum I/O operation size in bytes (default: 1GB)
	MaxIOSize int64
	// IODirName is the directory name for I/O operations under /tmp (default: hotpod)
	// Must be lowercase alphanumeric with optional hyphens, no paths or special chars.
	IODirName string
	// EnablePprof enables pprof endpoints on a separate port (6060)
	EnablePprof bool
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Port:             8080,
		LogLevel:         "info",
		ShutdownTimeout:  30 * time.Second,
		RequestTimeout:   5 * time.Minute,
		MaxConcurrentOps: 100,
		MaxCPUDuration:   60 * time.Second,
		MaxMemorySize:    1 << 30, // 1GB
		MaxIOSize:        1 << 30, // 1GB
		IODirName:        "hotpod",
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
	if cfg.MaxCPUDuration, err = getEnvDuration("HOTPOD_MAX_CPU_DURATION", cfg.MaxCPUDuration); err != nil {
		return nil, err
	}
	if cfg.MaxMemorySize, err = getEnvSize("HOTPOD_MAX_MEMORY_SIZE", cfg.MaxMemorySize); err != nil {
		return nil, err
	}
	if cfg.MaxIOSize, err = getEnvSize("HOTPOD_MAX_IO_SIZE", cfg.MaxIOSize); err != nil {
		return nil, err
	}
	cfg.IODirName = getEnvString("HOTPOD_IO_DIR_NAME", cfg.IODirName)
	if cfg.EnablePprof, err = getEnvBool("HOTPOD_ENABLE_PPROF", cfg.EnablePprof); err != nil {
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

func getEnvSize(key string, defaultVal int64) (int64, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return defaultVal, nil
	}
	size, err := ParseSize(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return size, nil
}

type sizeSuffix struct {
	suffix string
	mult   int64
}

var sizeSuffixes = []sizeSuffix{
	{"TB", 1 << 40},
	{"GB", 1 << 30},
	{"MB", 1 << 20},
	{"KB", 1 << 10},
	{"B", 1},
}

// ParseSize parses a human-readable size string (e.g., "100MB", "1GB") into bytes.
// Supported suffixes: B, KB, MB, GB, TB (case-insensitive).
func ParseSize(s string) (int64, error) {
	if s == "" {
		return 0, errors.New("empty size string")
	}

	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)

	for _, sm := range sizeSuffixes {
		if strings.HasSuffix(s, sm.suffix) {
			numStr := strings.TrimSuffix(s, sm.suffix)
			numStr = strings.TrimSpace(numStr)
			n, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid size number: %w", err)
			}
			if n < 0 {
				return 0, errors.New("size cannot be negative")
			}
			if n > math.MaxInt64/sm.mult {
				return 0, errors.New("size overflow: value too large")
			}
			return n * sm.mult, nil
		}
	}

	// No suffix, treat as bytes
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size: %w", err)
	}
	if n < 0 {
		return 0, errors.New("size cannot be negative")
	}
	return n, nil
}

// IOPath returns the full path for I/O operations (/tmp/<IODirName>).
func (c *Config) IOPath() string {
	return filepath.Join(IOBasePath, c.IODirName)
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

	if c.MaxCPUDuration < 0 {
		return fmt.Errorf("max CPU duration must be non-negative, got %s", c.MaxCPUDuration)
	}

	if c.MaxMemorySize < 0 {
		return fmt.Errorf("max memory size must be non-negative, got %d", c.MaxMemorySize)
	}

	if c.MaxIOSize < 0 {
		return fmt.Errorf("max I/O size must be non-negative, got %d", c.MaxIOSize)
	}

	if err := validateIODirName(c.IODirName); err != nil {
		return err
	}

	return nil
}

// validateIODirName ensures the I/O directory name is safe.
// It must be non-empty, lowercase alphanumeric with optional hyphens,
// no slashes, no special characters, no URL-encoded sequences.
func validateIODirName(name string) error {
	if name == "" {
		return errors.New("I/O directory name must not be empty")
	}

	if strings.Contains(name, "%") {
		return errors.New("I/O directory name cannot contain URL-encoded sequences")
	}

	for i, r := range name {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		isHyphen := r == '-'

		if !isLower && !isDigit && !isHyphen {
			return fmt.Errorf("I/O directory name must be lowercase alphanumeric with hyphens only, got %q at position %d", string(r), i)
		}
	}

	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		return errors.New("I/O directory name cannot start or end with hyphen")
	}

	if len(name) > 64 {
		return errors.New("I/O directory name too long (max 64 characters)")
	}

	return nil
}
