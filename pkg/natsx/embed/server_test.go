package embed

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	if config.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", config.Host)
	}

	if config.Port != DefaultPort {
		t.Errorf("expected port %d, got %d", DefaultPort, config.Port)
	}

	if config.StorePath != "./data/nats" {
		t.Errorf("expected store path ./data/nats, got %s", config.StorePath)
	}

	if config.MaxMemory != DefaultMaxMemoryMB {
		t.Errorf("expected max memory %d, got %d", DefaultMaxMemoryMB, config.MaxMemory)
	}

	if config.MaxStorage != DefaultMaxStorageGB {
		t.Errorf("expected max storage %d, got %d", DefaultMaxStorageGB, config.MaxStorage)
	}

	if config.LogLevel != "INFO" {
		t.Errorf("expected log level INFO, got %s", config.LogLevel)
	}

	if config.WriteDeadline != DefaultWriteDeadline {
		t.Errorf("expected write deadline %v, got %v", DefaultWriteDeadline, config.WriteDeadline)
	}
}

// validateServerConfigFields checks that a server config has expected field values.
func validateServerConfigFields(
	t *testing.T, config ServerConfig, expectedHost string, expectedPort int, expectedMemory int64,
) {
	t.Helper()

	if config.Host != expectedHost {
		t.Errorf("expected host %s, got %s", expectedHost, config.Host)
	}

	if config.Port != expectedPort {
		t.Errorf("expected port %d, got %d", expectedPort, config.Port)
	}

	if config.MaxMemory != expectedMemory {
		t.Errorf("expected max memory %d, got %d", expectedMemory, config.MaxMemory)
	}
}

// validateServerConfigDefaults checks that default values are properly set.
func validateServerConfigDefaults(t *testing.T, config ServerConfig) {
	t.Helper()

	if config.MaxStorage <= 0 {
		t.Error("MaxStorage should be set to default positive value")
	}

	if config.StorePath == "" {
		t.Error("StorePath should be set to default value")
	}

	if config.WriteDeadline <= 0 {
		t.Error("WriteDeadline should be set to default positive value")
	}
}

func TestServerConfig_Validate(t *testing.T) {
	tests := []struct {
		name           string
		config         ServerConfig
		expectedHost   string
		expectedPort   int
		expectedMemory int64
	}{
		{
			name: "default values",
			config: ServerConfig{
				Host:          "",
				Port:          0,
				MaxMemory:     0,
				MaxStorage:    0,
				StorePath:     "",
				WriteDeadline: 0,
			},
			expectedHost:   "127.0.0.1",
			expectedPort:   DefaultPort,
			expectedMemory: DefaultMaxMemoryMB,
		},
		{
			name: "custom valid values",
			config: ServerConfig{
				Host:          "localhost",
				Port:          4223,
				MaxMemory:     128 * 1024 * 1024,
				MaxStorage:    2 * 1024 * 1024 * 1024,
				StorePath:     "/tmp/nats",
				WriteDeadline: 5 * time.Second,
			},
			expectedHost:   "localhost",
			expectedPort:   4223,
			expectedMemory: 128 * 1024 * 1024,
		},
		{
			name: "random port",
			config: ServerConfig{
				Port: -1,
			},
			expectedHost:   "127.0.0.1",
			expectedPort:   0,
			expectedMemory: DefaultMaxMemoryMB,
		},
		{
			name: "invalid high port",
			config: ServerConfig{
				Port: 99999,
			},
			expectedHost:   "127.0.0.1",
			expectedPort:   DefaultPort,
			expectedMemory: DefaultMaxMemoryMB,
		},
		{
			name: "negative port",
			config: ServerConfig{
				Port: -100,
			},
			expectedHost:   "127.0.0.1",
			expectedPort:   DefaultPort,
			expectedMemory: DefaultMaxMemoryMB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err != nil {
				t.Errorf("Validate() error = %v", err)
			}

			validateServerConfigFields(t, tt.config, tt.expectedHost, tt.expectedPort, tt.expectedMemory)
			validateServerConfigDefaults(t, tt.config)
		})
	}
}

// validateEmbeddedServer checks that an embedded server instance is properly initialized.
func validateEmbeddedServer(t *testing.T, server *EmbeddedServer) {
	t.Helper()

	if server == nil {
		t.Fatal("NewEmbeddedServer() returned nil server")
	}

	if server.server == nil {
		t.Error("server.server is nil")
	}

	if server.config == nil {
		t.Error("server.config is nil")
	}

	if server.logger == nil {
		t.Error("server.logger is nil")
	}

	// Test that server is not running initially
	if server.IsRunning() {
		t.Error("server should not be running initially")
	}
}

func TestNewEmbeddedServer(t *testing.T) {
	tests := []struct {
		name    string
		config  *ServerConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: false, // Should use default config
		},
		{
			name:    "default config",
			config:  DefaultServerConfig(),
			wantErr: false,
		},
		{
			name: "custom config",
			config: &ServerConfig{
				Host:          "127.0.0.1",
				Port:          0, // Use random port
				StorePath:     "./test-data",
				MaxMemory:     32 * 1024 * 1024,
				MaxStorage:    512 * 1024 * 1024,
				LogLevel:      "DEBUG",
				WriteDeadline: 3 * time.Second,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewEmbeddedServer(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewEmbeddedServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			validateEmbeddedServer(t, server)
		})
	}
}

func TestEmbeddedServer_URL(t *testing.T) {
	config := &ServerConfig{
		Host: "127.0.0.1",
		Port: 4222,
	}

	server, err := NewEmbeddedServer(config)
	if err != nil {
		t.Fatalf("NewEmbeddedServer() error = %v", err)
	}

	url := server.URL()
	expected := "nats://127.0.0.1:4222"

	if url != expected {
		t.Errorf("expected URL %s, got %s", expected, url)
	}

	// Test ClientURL (should be the same)
	clientURL := server.ClientURL()
	if clientURL != expected {
		t.Errorf("expected ClientURL %s, got %s", expected, clientURL)
	}
}

func TestEmbeddedServer_Stats_Nil(t *testing.T) {
	server := &EmbeddedServer{
		server: nil,
	}

	stats := server.Stats()
	if stats != nil {
		t.Error("expected nil stats when server is nil")
	}
}

func TestEmbeddedServer_IsRunning(t *testing.T) {
	server, err := NewEmbeddedServer(nil)
	if err != nil {
		t.Fatalf("NewEmbeddedServer() error = %v", err)
	}

	// Initially not running
	if server.IsRunning() {
		t.Error("server should not be running initially")
	}

	// Set stopped flag and test
	server.stopped = true
	if server.IsRunning() {
		t.Error("server should not be running when stopped flag is set")
	}
}

func TestEmbeddedServer_HealthCheck(t *testing.T) {
	server, err := NewEmbeddedServer(nil)
	if err != nil {
		t.Fatalf("NewEmbeddedServer() error = %v", err)
	}

	// Health check should fail when not running
	err = server.HealthCheck()
	if err == nil {
		t.Error("HealthCheck() should fail when server is not running")
	}

	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("expected error to contain 'not running', got: %v", err)
	}
}

func TestSafeConversions(t *testing.T) {
	// Test safeInt64ToUint64
	tests := []struct {
		name     string
		input    int64
		expected uint64
	}{
		{
			name:     "positive value",
			input:    123,
			expected: 123,
		},
		{
			name:     "zero value",
			input:    0,
			expected: 0,
		},
		{
			name:     "negative value",
			input:    -123,
			expected: 0,
		},
		{
			name:     "max int64",
			input:    MaxInt63,
			expected: uint64(MaxInt63),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeInt64ToUint64(tt.input)
			if result != tt.expected {
				t.Errorf("safeInt64ToUint64(%d) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}

	// Test safeUint64ToInt64
	uint64Tests := []struct {
		name     string
		input    uint64
		expected int64
	}{
		{
			name:     "small value",
			input:    123,
			expected: 123,
		},
		{
			name:     "zero value",
			input:    0,
			expected: 0,
		},
		{
			name:     "max safe value",
			input:    uint64(MaxInt63),
			expected: MaxInt63,
		},
		{
			name:     "overflow value",
			input:    uint64(MaxInt63) + 1,
			expected: MaxInt63,
		},
		{
			name:     "max uint64",
			input:    ^uint64(0), // max uint64
			expected: MaxInt63,
		},
	}

	for _, tt := range uint64Tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeUint64ToInt64(tt.input)
			if result != tt.expected {
				t.Errorf("safeUint64ToInt64(%d) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEmbeddedServer_Stop_NotRunning(t *testing.T) {
	server, err := NewEmbeddedServer(nil)
	if err != nil {
		t.Fatalf("NewEmbeddedServer() error = %v", err)
	}

	// Stop should succeed even when not running
	err = server.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// Calling stop multiple times should be safe
	err = server.Stop()
	if err != nil {
		t.Errorf("Stop() called twice error = %v", err)
	}
}

func TestEmbeddedServer_WaitForShutdown_NilServer(_ *testing.T) {
	server := &EmbeddedServer{
		server: nil,
	}

	// Should not panic
	server.WaitForShutdown()
}

func BenchmarkSafeInt64ToUint64(b *testing.B) {
	value := int64(123456)
	b.ResetTimer()
	for range b.N {
		_ = safeInt64ToUint64(value)
	}
}

func BenchmarkSafeUint64ToInt64(b *testing.B) {
	value := uint64(123456)
	b.ResetTimer()
	for range b.N {
		_ = safeUint64ToInt64(value)
	}
}
