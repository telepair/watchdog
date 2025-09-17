package client

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Name != "natsx.client" {
		t.Errorf("expected name to be 'natsx.client', got %s", config.Name)
	}

	if len(config.URLs) != 1 {
		t.Errorf("expected 1 URL, got %d", len(config.URLs))
	}

	if config.ConnectTimeout != DefaultConnectTimeout {
		t.Errorf("expected connect timeout to be %v, got %v", DefaultConnectTimeout, config.ConnectTimeout)
	}

	if config.MaxReconnects != UnlimitedReconnects {
		t.Errorf("expected max reconnects to be %d, got %d", UnlimitedReconnects, config.MaxReconnects)
	}

	if config.ReconnectWait != DefaultReconnectWait {
		t.Errorf("expected reconnect wait to be %v, got %v", DefaultReconnectWait, config.ReconnectWait)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name: "valid default config",
			config: &Config{
				Name:           "test-client",
				URLs:           []string{"nats://localhost:4222"},
				ConnectTimeout: 2 * time.Second,
				ReconnectWait:  2 * time.Second,
			},
			expectErr: false,
		},
		{
			name: "empty name gets default",
			config: &Config{
				URLs:           []string{"nats://localhost:4222"},
				ConnectTimeout: 2 * time.Second,
				ReconnectWait:  2 * time.Second,
			},
			expectErr: false,
		},
		{
			name: "empty URLs gets default",
			config: &Config{
				Name:           "test-client",
				ConnectTimeout: 2 * time.Second,
				ReconnectWait:  2 * time.Second,
			},
			expectErr: false,
		},
		{
			name: "invalid URL",
			config: &Config{
				Name:           "test-client",
				URLs:           []string{"invalid://localhost:4222"},
				ConnectTimeout: 2 * time.Second,
				ReconnectWait:  2 * time.Second,
			},
			expectErr: true,
		},
		{
			name: "zero timeout gets default",
			config: &Config{
				Name:          "test-client",
				URLs:          []string{"nats://localhost:4222"},
				ReconnectWait: 2 * time.Second,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.expectErr {
				t.Errorf("Validate() error = %v, expectErr %v", err, tt.expectErr)
			}

			if !tt.expectErr {
				// Check defaults were applied
				if tt.config.Name == "" {
					t.Error("Name should have been set to default")
				}
				if len(tt.config.URLs) == 0 {
					t.Error("URLs should have been set to default")
				}
				if tt.config.ConnectTimeout <= 0 {
					t.Error("ConnectTimeout should have been set to default")
				}
				if tt.config.ReconnectWait <= 0 {
					t.Error("ReconnectWait should have been set to default")
				}
			}
		})
	}
}

func TestCreateMetrics(t *testing.T) {
	tests := []struct {
		name       string
		collector  MetricsCollector
		clientName string
		expectNoop bool
	}{
		{
			name:       "with collector",
			collector:  &NoopCollector{},
			clientName: "test-client",
			expectNoop: false,
		},
		{
			name:       "without collector",
			collector:  nil,
			clientName: "test-client",
			expectNoop: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := createMetrics(tt.collector, tt.clientName)
			if metrics == nil {
				t.Error("createMetrics should never return nil")
			}

			// Check that we get the right type of metrics
			if tt.expectNoop {
				if _, ok := metrics.collector.(*NoopCollector); !ok {
					t.Error("expected NoopCollector when no collector provided")
				}
			}
		})
	}
}

func TestNewClient_InvalidConfig(t *testing.T) {
	// Test with invalid config (should fail during validation)
	invalidConfig := &Config{
		Name: "test-client",
		URLs: []string{"invalid://localhost:4222"},
	}

	client, err := NewClient(invalidConfig)
	if err == nil {
		t.Error("NewClient with invalid config should return error")
	}
	if client != nil {
		t.Error("NewClient with invalid config should return nil client")
		client.Close()
	}
}

func TestClient_CheckClientState(t *testing.T) {
	// Test with mock client that's closed
	client := &Client{
		closed:  true,
		metrics: NewNoopMetrics("test"),
	}

	err := client.CheckClientState()
	if err == nil {
		t.Error("CheckClientState should return error for closed client")
	}
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}

	// Test with mock client that's open but no connection
	client.closed = false
	client.conn = nil

	err = client.CheckClientState()
	if err == nil {
		t.Error("CheckClientState should return error for client with no connection")
	}
}

func TestClient_ErrorWithMetrics(t *testing.T) {
	// Create mock metrics collector
	mockCollector := &MockMetricsCollector{
		counters: make(map[string]int),
	}

	client := &Client{
		metrics: NewMetrics(mockCollector, "test"),
	}

	// Test with error
	testErr := context.DeadlineExceeded
	result := client.ErrorWithMetrics(testErr)

	if !errors.Is(result, testErr) {
		t.Errorf("ErrorWithMetrics should return the same error, got %v", result)
	}

	// Check that error was recorded in metrics
	errorKey := "natsx_client_test_errors_total"
	if mockCollector.counters[errorKey] != 1 {
		t.Errorf("expected error counter to be 1, got %d", mockCollector.counters[errorKey])
	}

	// Test with nil error
	result = client.ErrorWithMetrics(nil)
	if result != nil {
		t.Error("ErrorWithMetrics should return nil when given nil")
	}

	// Error counter should still be 1
	if mockCollector.counters[errorKey] != 1 {
		t.Errorf("expected error counter to remain 1, got %d", mockCollector.counters[errorKey])
	}
}

func TestWrapValidationError(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		err      error
		expected string
	}{
		{
			name:     "with error",
			field:    "subject",
			err:      ErrInvalidSubject,
			expected: "invalid subject: invalid subject",
		},
		{
			name:     "with nil error",
			field:    "subject",
			err:      nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapValidationError(tt.field, tt.err)
			if tt.expected == "" {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else {
				if result == nil {
					t.Error("expected error, got nil")
				} else if result.Error() != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result.Error())
				}
			}
		})
	}
}

// MockMetricsCollector is a simple mock for testing.
type MockMetricsCollector struct {
	counters   map[string]int
	gauges     map[string]float64
	histograms map[string][]float64
}

func (m *MockMetricsCollector) IncCounter(name string) {
	if m.counters == nil {
		m.counters = make(map[string]int)
	}
	m.counters[name]++
}

func (m *MockMetricsCollector) AddCounter(name string, value float64) {
	if m.counters == nil {
		m.counters = make(map[string]int)
	}
	m.counters[name] += int(value)
}

func (m *MockMetricsCollector) SetGauge(name string, value float64) {
	if m.gauges == nil {
		m.gauges = make(map[string]float64)
	}
	m.gauges[name] = value
}

func (m *MockMetricsCollector) RecordHistogram(name string, value float64) {
	if m.histograms == nil {
		m.histograms = make(map[string][]float64)
	}
	m.histograms[name] = append(m.histograms[name], value)
}
