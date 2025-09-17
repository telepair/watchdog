package client

import (
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestSetGlobalConfig(t *testing.T) {
	// Clean up any existing global state
	defer func() {
		_ = CloseGlobalClient()
		globalManager.config = nil
		globalManager.initialized = false
	}()

	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Name:           "test-global",
				URLs:           []string{"nats://localhost:4222"},
				ConnectTimeout: 2 * time.Second,
				ReconnectWait:  2 * time.Second,
			},
			expectErr: false,
		},
		{
			name:      "nil config",
			config:    nil,
			expectErr: true,
		},
		{
			name: "invalid config",
			config: &Config{
				Name: "test-global",
				URLs: []string{"invalid://localhost:4222"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetGlobalConfig(tt.config)
			if (err != nil) != tt.expectErr {
				t.Errorf("SetGlobalConfig() error = %v, expectErr %v", err, tt.expectErr)
			}

			if !tt.expectErr && globalManager.config == nil {
				t.Error("Expected global config to be set")
			}
		})
	}
}

func TestSetGlobalConfig_ReconfigureExisting(t *testing.T) {
	// Clean up any existing global state
	defer func() {
		_ = CloseGlobalClient()
		globalManager.config = nil
		globalManager.initialized = false
	}()

	// Set initial config
	config1 := &Config{
		Name:           "test-global-1",
		URLs:           []string{"nats://localhost:4222"},
		ConnectTimeout: 2 * time.Second,
		ReconnectWait:  2 * time.Second,
	}

	err := SetGlobalConfig(config1)
	if err != nil {
		t.Fatalf("Failed to set initial config: %v", err)
	}

	// Mock an initialized client to test reconfiguration
	globalManager.initialized = true
	globalManager.client = &Client{
		closed: false,
		config: config1,
		logger: slog.Default().With("component", "test-client"),
	}

	// Set new config
	config2 := &Config{
		Name:           "test-global-2",
		URLs:           []string{"nats://localhost:4223"},
		ConnectTimeout: 3 * time.Second,
		ReconnectWait:  3 * time.Second,
	}

	err = SetGlobalConfig(config2)
	if err != nil {
		t.Errorf("Failed to reconfigure: %v", err)
	}

	if globalManager.config.Name != "test-global-2" {
		t.Errorf("Expected config name to be 'test-global-2', got %s", globalManager.config.Name)
	}

	if globalManager.initialized {
		t.Error("Expected global manager to be uninitialized after reconfiguration")
	}
}

func TestIsGlobalClientConnected(t *testing.T) {
	// Clean up any existing global state
	defer func() {
		_ = CloseGlobalClient()
		globalManager.config = nil
		globalManager.initialized = false
		globalManager.client = nil
	}()

	// Test with no client
	connected := IsGlobalClientConnected()
	if connected {
		t.Error("Expected false when no client exists")
	}

	// Test with mock closed client
	globalManager.client = &Client{
		closed: true,
	}

	connected = IsGlobalClientConnected()
	if connected {
		t.Error("Expected false for closed client")
	}
}

func TestCloseGlobalClient(t *testing.T) {
	// Clean up any existing global state
	defer func() {
		globalManager.config = nil
		globalManager.initialized = false
		globalManager.client = nil
	}()

	// Test closing when no client exists
	err := CloseGlobalClient()
	if err != nil {
		t.Errorf("CloseGlobalClient should not error when no client exists: %v", err)
	}

	// Test closing with mock client
	mockClient := &Client{
		closed: false,
		logger: slog.Default().With("component", "test-client"),
	}

	globalManager.client = mockClient
	globalManager.initialized = true

	err = CloseGlobalClient()
	if err != nil {
		t.Errorf("CloseGlobalClient failed: %v", err)
	}

	if globalManager.client != nil {
		t.Error("Expected client to be nil after close")
	}

	if globalManager.initialized {
		t.Error("Expected initialized to be false after close")
	}
}

func TestGetGlobalClientStats(t *testing.T) {
	// Clean up any existing global state
	defer func() {
		_ = CloseGlobalClient()
		globalManager.config = nil
		globalManager.initialized = false
		globalManager.client = nil
	}()

	// Test with no config and no client
	stats := GetGlobalClientStats()
	if stats == nil {
		t.Fatal("GetGlobalClientStats should never return nil")
	}

	if stats.IsConnected {
		t.Error("Expected IsConnected to be false when no client")
	}

	if stats.Subscriptions != 0 {
		t.Errorf("Expected 0 subscriptions, got %d", stats.Subscriptions)
	}

	// Test with config but no client
	config := &Config{
		Name: "test-global",
		URLs: []string{"nats://localhost:4222", "nats://localhost:4223"},
	}
	globalManager.config = config

	stats = GetGlobalClientStats()
	if len(stats.ConfigURLs) != 2 {
		t.Errorf("Expected 2 config URLs, got %d", len(stats.ConfigURLs))
	}

	// Test with mock client (using actual Client struct without real connection)
	mockClient := &Client{
		closed: false,
		conn:   nil, // No real connection for unit test
		logger: slog.Default().With("component", "test-client"),
	}

	globalManager.client = mockClient

	stats = GetGlobalClientStats()
	// Since conn is nil, IsConnected should be false
	if stats.IsConnected {
		t.Error("Expected IsConnected to be false for client with no connection")
	}

	if stats.Subscriptions != 0 {
		t.Errorf("Expected 0 subscriptions, got %d", stats.Subscriptions)
	}
}

func TestGetConnection(t *testing.T) {
	// Clean up any existing global state
	defer func() {
		_ = CloseGlobalClient()
		globalManager.config = nil
		globalManager.initialized = false
		globalManager.client = nil
	}()

	// Test with no client
	conn := GetConnection()
	if conn != nil {
		t.Error("Expected nil connection when no client")
	}

	// Test with client that has no connection
	globalManager.client = &Client{
		closed: false,
		conn:   nil,
		logger: slog.Default().With("component", "test-client"),
	}

	conn = GetConnection()
	if conn != nil {
		t.Error("Expected nil connection for client with no connection")
	}

	// Test with closed client
	globalManager.client.closed = true
	conn = GetConnection()
	if conn != nil {
		t.Error("Expected nil connection for closed client")
	}
}

func TestGlobalManagerConcurrency(t *testing.T) {
	// Clean up any existing global state
	defer func() {
		_ = CloseGlobalClient()
		globalManager.config = nil
		globalManager.initialized = false
		globalManager.client = nil
	}()

	const numGoroutines = 10
	const iterations = 5

	config := &Config{
		Name:           "concurrent-test",
		URLs:           []string{"nats://localhost:4222"},
		ConnectTimeout: 2 * time.Second,
		ReconnectWait:  2 * time.Second,
	}

	err := SetGlobalConfig(config)
	if err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*iterations)

	// Test concurrent access to global manager
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				// These operations should be thread-safe
				_ = IsGlobalClientConnected()
				_ = GetGlobalClientStats()
				_ = GetConnection()

				// Try to close and reconfigure
				_ = CloseGlobalClient()
				if setErr := SetGlobalConfig(config); setErr != nil {
					errors <- setErr
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for any errors during concurrent access
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

func TestGlobalConnectionManager_initClient(t *testing.T) {
	// Clean up any existing global state
	defer func() {
		_ = CloseGlobalClient()
		globalManager.config = nil
		globalManager.initialized = false
		globalManager.client = nil
	}()

	// Test initClient with invalid config (should fail during connection)
	invalidConfig := &Config{
		Name:           "test-init",
		URLs:           []string{"nats://nonexistent-server:9999"},
		ConnectTimeout: 100 * time.Millisecond, // Short timeout to fail quickly
		ReconnectWait:  100 * time.Millisecond,
	}

	globalManager.config = invalidConfig

	client, err := globalManager.initClient()
	if err == nil {
		t.Error("Expected error when connecting to nonexistent server")
		if client != nil {
			client.Close()
		}
	}
	if client != nil {
		t.Error("Expected nil client when connection fails")
	}
}
