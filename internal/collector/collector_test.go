package collector

import (
	"context"
	"errors"
	"testing"

	"github.com/telepair/watchdog/internal/collector/types"
)

// mockPublisher is a mock implementation of the Publisher interface
type mockPublisher struct {
	publishErr error
	published  []struct {
		subject string
		data    interface{}
	}
}

func (m *mockPublisher) Publish(ctx context.Context, subject string, data interface{}) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.published = append(m.published, struct {
		subject string
		data    interface{}
	}{subject, data})
	return nil
}

// mockCollector is a mock implementation of the Collector interface
type mockCollector struct {
	name      string
	startErr  error
	stopErr   error
	healthErr error
	started   bool
	stopped   bool
}

func (m *mockCollector) Name() string {
	return m.name
}

func (m *mockCollector) Start() error {
	if m.startErr != nil {
		return m.startErr
	}
	m.started = true
	return nil
}

func (m *mockCollector) Stop() error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.stopped = true
	return nil
}

func (m *mockCollector) Health() error {
	return m.healthErr
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		reporter    types.Publisher
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid config and reporter",
			config:   &Config{},
			reporter: &mockPublisher{},
			wantErr:  false,
		},
		{
			name:        "nil config",
			config:      nil,
			reporter:    &mockPublisher{},
			wantErr:     true,
			errContains: "config is required",
		},
		{
			name:        "nil reporter",
			config:      &Config{},
			reporter:    nil,
			wantErr:     true,
			errContains: "reporter is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager("test-agent", tt.config, tt.reporter)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if manager == nil {
				t.Error("expected manager, got nil")
				return
			}

			if manager.cfg != tt.config {
				t.Error("config not set correctly")
			}

			if manager.reporter != tt.reporter {
				t.Error("reporter not set correctly")
			}

			if len(manager.collectors) == 0 {
				t.Error("expected at least one collector")
			}
		})
	}
}

func TestManager_Start(t *testing.T) {
	tests := []struct {
		name              string
		collectorStartErr error
		wantErr           bool
		errContains       string
	}{
		{
			name:              "successful start",
			collectorStartErr: nil,
			wantErr:           false,
		},
		{
			name:              "collector start error",
			collectorStartErr: errors.New("collector start failed"),
			wantErr:           true,
			errContains:       "collector start failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{}
			reporter := &mockPublisher{}

			manager, err := NewManager("test-agent", config, reporter)
			if err != nil {
				t.Fatalf("failed to create manager: %v", err)
			}

			// Replace collectors with mock
			mockCollector := &mockCollector{
				name:     "test-collector",
				startErr: tt.collectorStartErr,
			}
			manager.collectors = []types.Collector{mockCollector}

			err = manager.Start()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !mockCollector.started {
				t.Error("expected collector to be started")
			}
		})
	}
}

func TestManager_Stop(t *testing.T) {
	tests := []struct {
		name             string
		collectorStopErr error
		wantErr          bool
		errContains      string
	}{
		{
			name:             "successful stop",
			collectorStopErr: nil,
			wantErr:          false,
		},
		{
			name:             "collector stop error",
			collectorStopErr: errors.New("collector stop failed"),
			wantErr:          true,
			errContains:      "collector stop failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{}
			reporter := &mockPublisher{}

			manager, err := NewManager("test-agent", config, reporter)
			if err != nil {
				t.Fatalf("failed to create manager: %v", err)
			}

			// Replace collectors with mock
			mockCollector := &mockCollector{
				name:    "test-collector",
				stopErr: tt.collectorStopErr,
			}
			manager.collectors = []types.Collector{mockCollector}

			err = manager.Stop()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !mockCollector.stopped {
				t.Error("expected collector to be stopped")
			}
		})
	}
}

func TestManager_Health(t *testing.T) {
	tests := []struct {
		name               string
		collectorHealthErr error
		wantErr            bool
		errContains        string
	}{
		{
			name:               "healthy",
			collectorHealthErr: nil,
			wantErr:            false,
		},
		{
			name:               "unhealthy collector",
			collectorHealthErr: errors.New("collector unhealthy"),
			wantErr:            true,
			errContains:        "collector unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{}
			reporter := &mockPublisher{}

			manager, err := NewManager("test-agent", config, reporter)
			if err != nil {
				t.Fatalf("failed to create manager: %v", err)
			}

			// Replace collectors with mock
			mockCollector := &mockCollector{
				name:      "test-collector",
				healthErr: tt.collectorHealthErr,
			}
			manager.collectors = []types.Collector{mockCollector}

			err = manager.Health()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
		})
	}
}

func TestManager_MultipleCollectors(t *testing.T) {
	config := &Config{}
	reporter := &mockPublisher{}

	manager, err := NewManager("test-agent", config, reporter)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Add multiple mock collectors
	mockCollector1 := &mockCollector{name: "collector-1"}
	mockCollector2 := &mockCollector{name: "collector-2"}
	manager.collectors = []types.Collector{mockCollector1, mockCollector2}

	// Test Start
	err = manager.Start()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mockCollector1.started {
		t.Error("expected collector-1 to be started")
	}
	if !mockCollector2.started {
		t.Error("expected collector-2 to be started")
	}

	// Test Health
	err = manager.Health()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test Stop
	err = manager.Stop()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !mockCollector1.stopped {
		t.Error("expected collector-1 to be stopped")
	}
	if !mockCollector2.stopped {
		t.Error("expected collector-2 to be stopped")
	}
}

func TestManager_CollectorStartFailure(t *testing.T) {
	config := &Config{}
	reporter := &mockPublisher{}

	manager, err := NewManager("test-agent", config, reporter)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Add collectors where one fails to start
	mockCollector1 := &mockCollector{name: "collector-1"}
	mockCollector2 := &mockCollector{name: "collector-2", startErr: errors.New("start failed")}
	manager.collectors = []types.Collector{mockCollector1, mockCollector2}

	// Test Start - should fail
	err = manager.Start()
	if err == nil {
		t.Error("expected error, got nil")
	}

	// First collector should be started, second should not
	if !mockCollector1.started {
		t.Error("expected collector-1 to be started")
	}
	if mockCollector2.started {
		t.Error("expected collector-2 to not be started")
	}
}

func TestManager_CollectorStopFailure(t *testing.T) {
	config := &Config{}
	reporter := &mockPublisher{}

	manager, err := NewManager("test-agent", config, reporter)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Add collectors where one fails to stop
	mockCollector1 := &mockCollector{name: "collector-1"}
	mockCollector2 := &mockCollector{name: "collector-2", stopErr: errors.New("stop failed")}
	manager.collectors = []types.Collector{mockCollector1, mockCollector2}

	// Start collectors first
	err = manager.Start()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test Stop - should fail
	err = manager.Stop()
	if err == nil {
		t.Error("expected error, got nil")
	}

	// First collector should be stopped, second should not due to error
	if !mockCollector1.stopped {
		t.Error("expected collector-1 to be stopped")
	}
	if mockCollector2.stopped {
		t.Error("expected collector-2 to not be stopped due to error")
	}
}

func TestManager_CollectorHealthFailure(t *testing.T) {
	config := &Config{}
	reporter := &mockPublisher{}

	manager, err := NewManager("test-agent", config, reporter)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Add collectors where one fails health check
	mockCollector1 := &mockCollector{name: "collector-1"}
	mockCollector2 := &mockCollector{name: "collector-2", healthErr: errors.New("health failed")}
	manager.collectors = []types.Collector{mockCollector1, mockCollector2}

	// Test Health - should fail
	err = manager.Health()
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			contains(s[1:], substr))))
}

// Benchmark tests
func BenchmarkManager_Start(b *testing.B) {
	config := &Config{}
	reporter := &mockPublisher{}

	manager, err := NewManager("test-agent", config, reporter)
	if err != nil {
		b.Fatalf("failed to create manager: %v", err)
	}

	// Replace collectors with mock
	mockCollector := &mockCollector{name: "test-collector"}
	manager.collectors = []types.Collector{mockCollector}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.Start()
	}
}

func BenchmarkManager_Stop(b *testing.B) {
	config := &Config{}
	reporter := &mockPublisher{}

	manager, err := NewManager("test-agent", config, reporter)
	if err != nil {
		b.Fatalf("failed to create manager: %v", err)
	}

	// Replace collectors with mock
	mockCollector := &mockCollector{name: "test-collector"}
	manager.collectors = []types.Collector{mockCollector}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.Stop()
	}
}

func BenchmarkManager_Health(b *testing.B) {
	config := &Config{}
	reporter := &mockPublisher{}

	manager, err := NewManager("test-agent", config, reporter)
	if err != nil {
		b.Fatalf("failed to create manager: %v", err)
	}

	// Replace collectors with mock
	mockCollector := &mockCollector{name: "test-collector"}
	manager.collectors = []types.Collector{mockCollector}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.Health()
	}
}
