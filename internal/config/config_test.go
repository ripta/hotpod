package config

import (
	"os"
	"testing"
	"time"
)

type portValidationTest struct {
	port    int
	wantErr bool
}

var portValidationTests = []portValidationTest{
	{0, true},
	{1, false},
	{8080, false},
	{65535, false},
	{65536, true},
	{-1, true},
}

type logLevelValidationTest struct {
	level   string
	wantErr bool
}

var logLevelValidationTests = []logLevelValidationTest{
	{"debug", false},
	{"info", false},
	{"warn", false},
	{"error", false},
	{"invalid", true},
	{"INFO", true},
}

type negativeDurationTest struct {
	name string
	cfg  Config
}

var negativeDurationTests = []negativeDurationTest{
	{"StartupDelay", Config{Port: 8080, LogLevel: "info", StartupDelay: -1}},
	{"StartupJitter", Config{Port: 8080, LogLevel: "info", StartupJitter: -1}},
	{"ShutdownDelay", Config{Port: 8080, LogLevel: "info", ShutdownDelay: -1}},
	{"ShutdownTimeout", Config{Port: 8080, LogLevel: "info", ShutdownTimeout: -1}},
	{"RequestTimeout", Config{Port: 8080, LogLevel: "info", RequestTimeout: -1}},
}

func TestLoadDefaults(t *testing.T) {
	// Clear any existing env vars
	for _, key := range []string{
		"HOTPOD_PORT", "HOTPOD_LOG_LEVEL", "HOTPOD_STARTUP_DELAY",
		"HOTPOD_STARTUP_JITTER", "HOTPOD_SHUTDOWN_DELAY", "HOTPOD_SHUTDOWN_TIMEOUT",
		"HOTPOD_DRAIN_IMMEDIATELY", "HOTPOD_REQUEST_TIMEOUT",
	} {
		os.Unsetenv(key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want \"info\"", cfg.LogLevel)
	}
	if cfg.StartupDelay != 0 {
		t.Errorf("StartupDelay = %v, want 0", cfg.StartupDelay)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s", cfg.ShutdownTimeout)
	}
	if cfg.RequestTimeout != 5*time.Minute {
		t.Errorf("RequestTimeout = %v, want 5m", cfg.RequestTimeout)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("HOTPOD_PORT", "9000")
	os.Setenv("HOTPOD_LOG_LEVEL", "debug")
	os.Setenv("HOTPOD_STARTUP_DELAY", "5s")
	os.Setenv("HOTPOD_STARTUP_JITTER", "2s")
	os.Setenv("HOTPOD_SHUTDOWN_DELAY", "10s")
	os.Setenv("HOTPOD_SHUTDOWN_TIMEOUT", "60s")
	os.Setenv("HOTPOD_DRAIN_IMMEDIATELY", "true")
	os.Setenv("HOTPOD_REQUEST_TIMEOUT", "10m")
	defer func() {
		for _, key := range []string{
			"HOTPOD_PORT", "HOTPOD_LOG_LEVEL", "HOTPOD_STARTUP_DELAY",
			"HOTPOD_STARTUP_JITTER", "HOTPOD_SHUTDOWN_DELAY", "HOTPOD_SHUTDOWN_TIMEOUT",
			"HOTPOD_DRAIN_IMMEDIATELY", "HOTPOD_REQUEST_TIMEOUT",
		} {
			os.Unsetenv(key)
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want 9000", cfg.Port)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want \"debug\"", cfg.LogLevel)
	}
	if cfg.StartupDelay != 5*time.Second {
		t.Errorf("StartupDelay = %v, want 5s", cfg.StartupDelay)
	}
	if cfg.StartupJitter != 2*time.Second {
		t.Errorf("StartupJitter = %v, want 2s", cfg.StartupJitter)
	}
	if cfg.ShutdownDelay != 10*time.Second {
		t.Errorf("ShutdownDelay = %v, want 10s", cfg.ShutdownDelay)
	}
	if cfg.ShutdownTimeout != 60*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 60s", cfg.ShutdownTimeout)
	}
	if !cfg.DrainImmediately {
		t.Error("DrainImmediately = false, want true")
	}
	if cfg.RequestTimeout != 10*time.Minute {
		t.Errorf("RequestTimeout = %v, want 10m", cfg.RequestTimeout)
	}
}

func TestLoadInvalidPort(t *testing.T) {
	os.Setenv("HOTPOD_PORT", "invalid")
	defer os.Unsetenv("HOTPOD_PORT")

	_, err := Load()
	if err == nil {
		t.Error("Load() expected error for invalid port")
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	os.Setenv("HOTPOD_STARTUP_DELAY", "not-a-duration")
	defer os.Unsetenv("HOTPOD_STARTUP_DELAY")

	_, err := Load()
	if err == nil {
		t.Error("Load() expected error for invalid duration")
	}
}

func TestValidatePortRange(t *testing.T) {
	for _, tt := range portValidationTests {
		cfg := &Config{Port: tt.port, LogLevel: "info"}
		err := cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate() port=%d, error=%v, wantErr=%v", tt.port, err, tt.wantErr)
		}
	}
}

func TestValidateLogLevel(t *testing.T) {
	for _, tt := range logLevelValidationTests {
		cfg := &Config{Port: 8080, LogLevel: tt.level}
		err := cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate() level=%q, error=%v, wantErr=%v", tt.level, err, tt.wantErr)
		}
	}
}

func TestValidateNegativeDurations(t *testing.T) {
	base := Config{Port: 8080, LogLevel: "info"}
	if err := base.Validate(); err != nil {
		t.Fatalf("base config invalid: %v", err)
	}

	for _, tt := range negativeDurationTests {
		if err := tt.cfg.Validate(); err == nil {
			t.Errorf("Validate() %s=-1 should error", tt.name)
		}
	}
}

type parseSizeTest struct {
	input   string
	want    int64
	wantErr bool
}

var parseSizeTests = []parseSizeTest{
	{"100", 100, false},
	{"0", 0, false},
	{"1B", 1, false},
	{"100B", 100, false},
	{"1KB", 1024, false},
	{"1kb", 1024, false},
	{"10KB", 10240, false},
	{"1MB", 1 << 20, false},
	{"1GB", 1 << 30, false},
	{"1TB", 1 << 40, false},
	{"  100MB  ", 100 << 20, false},
	{"", 0, true},
	{"invalid", 0, true},
	{"-1", 0, true},
	{"-1MB", 0, true},
	{"1XB", 0, true},
	{"9999999999999999TB", 0, true},
}

func TestParseSize(t *testing.T) {
	for _, tt := range parseSizeTests {
		got, err := ParseSize(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseSize(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestLoadMaxCPUDurationDefault(t *testing.T) {
	os.Unsetenv("HOTPOD_MAX_CPU_DURATION")
	os.Unsetenv("HOTPOD_MAX_MEMORY_SIZE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.MaxCPUDuration != 60*time.Second {
		t.Errorf("MaxCPUDuration = %v, want 60s", cfg.MaxCPUDuration)
	}
	if cfg.MaxMemorySize != 1<<30 {
		t.Errorf("MaxMemorySize = %d, want %d (1GB)", cfg.MaxMemorySize, 1<<30)
	}
}

func TestLoadMaxCPUDurationFromEnv(t *testing.T) {
	os.Setenv("HOTPOD_MAX_CPU_DURATION", "30s")
	os.Setenv("HOTPOD_MAX_MEMORY_SIZE", "512MB")
	defer os.Unsetenv("HOTPOD_MAX_CPU_DURATION")
	defer os.Unsetenv("HOTPOD_MAX_MEMORY_SIZE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.MaxCPUDuration != 30*time.Second {
		t.Errorf("MaxCPUDuration = %v, want 30s", cfg.MaxCPUDuration)
	}
	if cfg.MaxMemorySize != 512<<20 {
		t.Errorf("MaxMemorySize = %d, want %d (512MB)", cfg.MaxMemorySize, 512<<20)
	}
}
