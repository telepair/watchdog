package health

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Addr != DefaultAddr {
		t.Errorf("expected addr %s, got %s", DefaultAddr, config.Addr)
	}

	if config.LivezPath != DefaultLivezPath {
		t.Errorf("expected livez path %s, got %s", DefaultLivezPath, config.LivezPath)
	}

	if config.ReadyzPath != DefaultReadyzPath {
		t.Errorf("expected readyz path %s, got %s", DefaultReadyzPath, config.ReadyzPath)
	}

	if config.MetricsPath != DefaultMetricsPath {
		t.Errorf("expected metrics path %s, got %s", DefaultMetricsPath, config.MetricsPath)
	}

	if config.MetricsNamespace != "" {
		t.Errorf("expected empty metrics namespace, got %s", config.MetricsNamespace)
	}
}

func TestConfig_Parse(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "default config",
			config:  *DefaultConfig(),
			wantErr: false,
		},
		{
			name: "valid custom config",
			config: Config{
				Addr:        ":8080",
				LivezPath:   "/health/live",
				ReadyzPath:  "/health/ready",
				MetricsPath: "/metrics",
			},
			wantErr: false,
		},
		{
			name: "paths without leading slash",
			config: Config{
				Addr:        ":8080",
				LivezPath:   "livez",
				ReadyzPath:  "readyz",
				MetricsPath: "metrics",
			},
			wantErr: false,
		},
		{
			name: "empty addr gets default",
			config: Config{
				Addr:        "",
				LivezPath:   "/livez",
				ReadyzPath:  "/readyz",
				MetricsPath: "/metrics",
			},
			wantErr: false, // applyDefaults will set a default addr
		},
		{
			name: "invalid addr - malformed",
			config: Config{
				Addr:        "invalid-addr",
				LivezPath:   "/livez",
				ReadyzPath:  "/readyz",
				MetricsPath: "/metrics",
			},
			wantErr: true,
		},
		{
			name: "duplicate paths - livez and readyz",
			config: Config{
				Addr:        ":8080",
				LivezPath:   "/health",
				ReadyzPath:  "/health",
				MetricsPath: "/metrics",
			},
			wantErr: true,
		},
		{
			name: "duplicate paths - all same",
			config: Config{
				Addr:        ":8080",
				LivezPath:   "/endpoint",
				ReadyzPath:  "/endpoint",
				MetricsPath: "/endpoint",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Parse()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Parse() error = %v, wantErr %v", err, tt.wantErr)
			}

			// If no error, verify paths are normalized
			if err == nil {
				if !strings.HasPrefix(tt.config.LivezPath, "/") {
					t.Error("livez path should start with /")
				}
				if !strings.HasPrefix(tt.config.ReadyzPath, "/") {
					t.Error("readyz path should start with /")
				}
				if !strings.HasPrefix(tt.config.MetricsPath, "/") {
					t.Error("metrics path should start with /")
				}
			}
		})
	}
}

func TestConfig_applyDefaults(t *testing.T) {
	config := &Config{}
	config.applyDefaults()

	if config.Addr != DefaultAddr {
		t.Errorf("expected addr %s, got %s", DefaultAddr, config.Addr)
	}

	if config.LivezPath != DefaultLivezPath {
		t.Errorf("expected livez path %s, got %s", DefaultLivezPath, config.LivezPath)
	}

	if config.ReadyzPath != DefaultReadyzPath {
		t.Errorf("expected readyz path %s, got %s", DefaultReadyzPath, config.ReadyzPath)
	}

	if config.MetricsPath != DefaultMetricsPath {
		t.Errorf("expected metrics path %s, got %s", DefaultMetricsPath, config.MetricsPath)
	}

	// Test that existing values are preserved
	config2 := &Config{
		Addr:        ":9090",
		LivezPath:   "/custom-livez",
		ReadyzPath:  "/custom-readyz",
		MetricsPath: "/custom-metrics",
	}
	config2.applyDefaults()

	if config2.Addr != ":9090" {
		t.Error("existing addr should be preserved")
	}

	if config2.LivezPath != "/custom-livez" {
		t.Error("existing livez path should be preserved")
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already normalized",
			input:    "/health",
			expected: "/health",
		},
		{
			name:     "missing slash",
			input:    "health",
			expected: "/health",
		},
		{
			name:     "empty path",
			input:    "",
			expected: "/",
		},
		{
			name:     "complex path",
			input:    "api/v1/health",
			expected: "/api/v1/health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePath(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateAddr(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{
			name:    "valid port only",
			addr:    ":8080",
			wantErr: false,
		},
		{
			name:    "valid host and port",
			addr:    "localhost:8080",
			wantErr: false,
		},
		{
			name:    "valid IP and port",
			addr:    "127.0.0.1:8080",
			wantErr: false,
		},
		{
			name:    "empty addr",
			addr:    "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			addr:    "   ",
			wantErr: true,
		},
		{
			name:    "invalid format",
			addr:    "invalid",
			wantErr: true,
		},
		{
			name:    "port too high",
			addr:    ":99999",
			wantErr: true,
		},
		{
			name:    "negative port",
			addr:    ":-1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAddr(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAddr(%s) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUniquePaths(t *testing.T) {
	tests := []struct {
		name        string
		livezPath   string
		readyzPath  string
		metricsPath string
		wantErr     bool
	}{
		{
			name:        "all unique paths",
			livezPath:   "/livez",
			readyzPath:  "/readyz",
			metricsPath: "/metrics",
			wantErr:     false,
		},
		{
			name:        "paths without leading slash",
			livezPath:   "livez",
			readyzPath:  "readyz",
			metricsPath: "metrics",
			wantErr:     false,
		},
		{
			name:        "livez and readyz duplicate",
			livezPath:   "/health",
			readyzPath:  "/health",
			metricsPath: "/metrics",
			wantErr:     true,
		},
		{
			name:        "livez and metrics duplicate",
			livezPath:   "/endpoint",
			readyzPath:  "/readyz",
			metricsPath: "/endpoint",
			wantErr:     true,
		},
		{
			name:        "readyz and metrics duplicate",
			livezPath:   "/livez",
			readyzPath:  "/endpoint",
			metricsPath: "/endpoint",
			wantErr:     true,
		},
		{
			name:        "all paths duplicate",
			livezPath:   "/same",
			readyzPath:  "/same",
			metricsPath: "/same",
			wantErr:     true,
		},
		{
			name:        "normalization makes duplicates",
			livezPath:   "/health",
			readyzPath:  "health",
			metricsPath: "/metrics",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUniquePaths(tt.livezPath, tt.readyzPath, tt.metricsPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateUniquePaths() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
