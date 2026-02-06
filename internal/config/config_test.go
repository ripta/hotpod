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
	{"StartupDelay", Config{Port: 8080, LogLevel: "info", IODirName: "test", Mode: "app", StartupDelay: -1}},
	{"StartupJitter", Config{Port: 8080, LogLevel: "info", IODirName: "test", Mode: "app", StartupJitter: -1}},
	{"ShutdownDelay", Config{Port: 8080, LogLevel: "info", IODirName: "test", Mode: "app", ShutdownDelay: -1}},
	{"ShutdownTimeout", Config{Port: 8080, LogLevel: "info", IODirName: "test", Mode: "app", ShutdownTimeout: -1}},
	{"RequestTimeout", Config{Port: 8080, LogLevel: "info", IODirName: "test", Mode: "app", RequestTimeout: -1}},
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
		cfg := &Config{Port: tt.port, LogLevel: "info", IODirName: "test", Mode: "app"}
		err := cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate() port=%d, error=%v, wantErr=%v", tt.port, err, tt.wantErr)
		}
	}
}

func TestValidateLogLevel(t *testing.T) {
	for _, tt := range logLevelValidationTests {
		cfg := &Config{Port: 8080, LogLevel: tt.level, IODirName: "test", Mode: "app"}
		err := cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate() level=%q, error=%v, wantErr=%v", tt.level, err, tt.wantErr)
		}
	}
}

func TestValidateNegativeDurations(t *testing.T) {
	base := Config{Port: 8080, LogLevel: "info", IODirName: "test", Mode: "app"}
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

type ioDirNameValidationTest struct {
	name    string
	wantErr bool
}

var ioDirNameValidationTests = []ioDirNameValidationTest{
	// Valid names
	{"hotpod", false},
	{"test", false},
	{"my-app", false},
	{"app123", false},
	{"a", false},
	{"abc-def-123", false},

	// Invalid: empty
	{"", true},

	// Invalid: contains slashes (path traversal)
	{"/tmp", true},
	{"../etc", true},
	{"foo/bar", true},
	{"a/b", true},

	// Invalid: uppercase
	{"Hotpod", true},
	{"HOTPOD", true},
	{"myApp", true},

	// Invalid: special characters
	{"hot_pod", true},
	{"hot.pod", true},
	{"hot pod", true},
	{"hot@pod", true},
	{"hot$pod", true},

	// Invalid: URL-encoded sequences
	{"%2e%2e", true},
	{"foo%2fbar", true},
	{"test%00null", true},

	// Invalid: starts/ends with hyphen
	{"-hotpod", true},
	{"hotpod-", true},
	{"-", true},

	// Invalid: too long
	{"abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz01234", true},
}

func TestValidateIODirName(t *testing.T) {
	for _, tt := range ioDirNameValidationTests {
		cfg := &Config{Port: 8080, LogLevel: "info", IODirName: tt.name, Mode: "app"}
		err := cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate() IODirName=%q, error=%v, wantErr=%v", tt.name, err, tt.wantErr)
		}
	}
}

func TestIOPath(t *testing.T) {
	cfg := &Config{IODirName: "myapp"}
	want := "/tmp/myapp"
	if got := cfg.IOPath(); got != want {
		t.Errorf("IOPath() = %q, want %q", got, want)
	}
}

type parseCPUTest struct {
	input   string
	want    time.Duration
	wantErr bool
}

var parseCPUTests = []parseCPUTest{
	{"100m", 100 * time.Millisecond, false},
	{"0m", 0, false},
	{"500m", 500 * time.Millisecond, false},
	{"1000m", 1000 * time.Millisecond, false},
	{"1500m", 1500 * time.Millisecond, false},
	{"0.5", 500 * time.Millisecond, false},
	{"1", 1 * time.Second, false},
	{"0", 0, false},
	{"0.1", 100 * time.Millisecond, false},
	{"  100m  ", 100 * time.Millisecond, false},
	{"", 0, true},
	{"-100m", 0, true},
	{"-0.5", 0, true},
	{"invalid", 0, true},
	{"abc m", 0, true},
}

func TestParseCPU(t *testing.T) {
	for _, tt := range parseCPUTests {
		got, err := ParseCPU(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseCPU(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseCPU(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

type parseSizeK8sTest struct {
	input string
	want  int64
}

var parseSizeK8sTests = []parseSizeK8sTest{
	{"1Ki", 1 << 10},
	{"1Mi", 1 << 20},
	{"1Gi", 1 << 30},
	{"1Ti", 1 << 40},
	{"50Mi", 50 << 20},
	{"100Ki", 100 << 10},
}

func TestParseSizeKubernetesSuffixes(t *testing.T) {
	for _, tt := range parseSizeK8sTests {
		got, err := ParseSize(tt.input)
		if err != nil {
			t.Errorf("ParseSize(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

type modeValidationTest struct {
	mode    string
	wantErr bool
}

var modeValidationTests = []modeValidationTest{
	{"app", false},
	{"sidecar", false},
	{"", true},
	{"invalid", true},
	{"APP", true},
	{"SIDECAR", true},
}

func TestValidateMode(t *testing.T) {
	for _, tt := range modeValidationTests {
		cfg := &Config{Port: 8080, LogLevel: "info", IODirName: "test", Mode: tt.mode}
		err := cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate() Mode=%q, error=%v, wantErr=%v", tt.mode, err, tt.wantErr)
		}
	}
}

func TestLoadSidecarConfig(t *testing.T) {
	os.Setenv("HOTPOD_MODE", "sidecar")
	os.Setenv("HOTPOD_SIDECAR_CPU_BASELINE", "200m")
	os.Setenv("HOTPOD_SIDECAR_CPU_JITTER", "20m")
	os.Setenv("HOTPOD_SIDECAR_MEMORY_BASELINE", "100Mi")
	os.Setenv("HOTPOD_SIDECAR_REQUEST_OVERHEAD", "5m")
	defer func() {
		for _, key := range []string{
			"HOTPOD_MODE", "HOTPOD_SIDECAR_CPU_BASELINE",
			"HOTPOD_SIDECAR_CPU_JITTER", "HOTPOD_SIDECAR_MEMORY_BASELINE",
			"HOTPOD_SIDECAR_REQUEST_OVERHEAD",
		} {
			os.Unsetenv(key)
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Mode != "sidecar" {
		t.Errorf("Mode = %q, want \"sidecar\"", cfg.Mode)
	}
	if cfg.SidecarCPUBaseline != 200*time.Millisecond {
		t.Errorf("SidecarCPUBaseline = %v, want 200ms", cfg.SidecarCPUBaseline)
	}
	if cfg.SidecarCPUJitter != 20*time.Millisecond {
		t.Errorf("SidecarCPUJitter = %v, want 20ms", cfg.SidecarCPUJitter)
	}
	if cfg.SidecarMemoryBaseline != 100<<20 {
		t.Errorf("SidecarMemoryBaseline = %d, want %d (100Mi)", cfg.SidecarMemoryBaseline, 100<<20)
	}
	if cfg.SidecarRequestOverhead != 5*time.Millisecond {
		t.Errorf("SidecarRequestOverhead = %v, want 5ms", cfg.SidecarRequestOverhead)
	}
}

func TestLoadSidecarDefaults(t *testing.T) {
	os.Unsetenv("HOTPOD_MODE")
	os.Unsetenv("HOTPOD_SIDECAR_CPU_BASELINE")
	os.Unsetenv("HOTPOD_SIDECAR_CPU_JITTER")
	os.Unsetenv("HOTPOD_SIDECAR_MEMORY_BASELINE")
	os.Unsetenv("HOTPOD_SIDECAR_REQUEST_OVERHEAD")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Mode != "app" {
		t.Errorf("Mode = %q, want \"app\"", cfg.Mode)
	}
	if cfg.SidecarCPUBaseline != 100*time.Millisecond {
		t.Errorf("SidecarCPUBaseline = %v, want 100ms", cfg.SidecarCPUBaseline)
	}
	if cfg.SidecarCPUJitter != 10*time.Millisecond {
		t.Errorf("SidecarCPUJitter = %v, want 10ms", cfg.SidecarCPUJitter)
	}
	if cfg.SidecarMemoryBaseline != 50<<20 {
		t.Errorf("SidecarMemoryBaseline = %d, want %d (50Mi)", cfg.SidecarMemoryBaseline, 50<<20)
	}
	if cfg.SidecarRequestOverhead != 0 {
		t.Errorf("SidecarRequestOverhead = %v, want 0", cfg.SidecarRequestOverhead)
	}
}

func TestValidateSidecarCPUBaselineRange(t *testing.T) {
	base := Config{Port: 8080, LogLevel: "info", IODirName: "test", Mode: "sidecar"}

	// Valid: 0
	base.SidecarCPUBaseline = 0
	if err := base.Validate(); err != nil {
		t.Errorf("Validate() baseline=0 should not error: %v", err)
	}

	// Valid: 1s
	base.SidecarCPUBaseline = time.Second
	if err := base.Validate(); err != nil {
		t.Errorf("Validate() baseline=1s should not error: %v", err)
	}

	// Invalid: >1s
	base.SidecarCPUBaseline = time.Second + time.Millisecond
	if err := base.Validate(); err == nil {
		t.Error("Validate() baseline>1s should error")
	}

	// Invalid: negative
	base.SidecarCPUBaseline = -1
	if err := base.Validate(); err == nil {
		t.Error("Validate() baseline<0 should error")
	}
}
