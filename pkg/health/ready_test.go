package health

import (
	"log/slog"
	"testing"
	"time"
)

func TestNewReadinessManager(t *testing.T) {
	rm := NewReadinessManager()

	if rm == nil {
		t.Fatal("expected ReadinessManager, got nil")
	}

	if rm.IsReady() {
		t.Error("expected initial ready state to be false")
	}

	if rm.logger == nil {
		t.Error("expected logger to be initialized")
	}

	if rm.startTime.IsZero() {
		t.Error("expected start time to be set")
	}

	// Verify start time is recent (within last second)
	if time.Since(rm.startTime) > time.Second {
		t.Error("expected start time to be recent")
	}
}

func TestReadinessManager_SetReady(t *testing.T) {
	tests := []struct {
		name         string
		initialState bool
		newState     bool
		expectChange bool
	}{
		{
			name:         "set ready from false to true",
			initialState: false,
			newState:     true,
			expectChange: true,
		},
		{
			name:         "set ready from true to false",
			initialState: true,
			newState:     false,
			expectChange: true,
		},
		{
			name:         "set ready from false to false",
			initialState: false,
			newState:     false,
			expectChange: false,
		},
		{
			name:         "set ready from true to true",
			initialState: true,
			newState:     true,
			expectChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := NewReadinessManager()

			// Set initial state
			rm.ready.Store(tt.initialState)

			// Verify initial state
			if rm.IsReady() != tt.initialState {
				t.Fatalf("failed to set initial state to %v", tt.initialState)
			}

			// Set new state
			rm.SetReady(tt.newState)

			// Verify new state
			if rm.IsReady() != tt.newState {
				t.Errorf("expected ready state %v, got %v", tt.newState, rm.IsReady())
			}
		})
	}
}

func TestReadinessManager_IsReady(t *testing.T) {
	rm := NewReadinessManager()

	// Test initial state
	if rm.IsReady() {
		t.Error("expected initial ready state to be false")
	}

	// Test setting to true
	rm.SetReady(true)
	if !rm.IsReady() {
		t.Error("expected ready state to be true after SetReady(true)")
	}

	// Test setting to false
	rm.SetReady(false)
	if rm.IsReady() {
		t.Error("expected ready state to be false after SetReady(false)")
	}
}

func TestReadinessManager_GetUptime(t *testing.T) {
	rm := NewReadinessManager()

	// Test immediately after creation
	uptime1 := rm.GetUptime()
	if uptime1 < 0 {
		t.Error("expected uptime to be non-negative")
	}

	// Wait a small amount and check uptime increased
	time.Sleep(10 * time.Millisecond)
	uptime2 := rm.GetUptime()

	if uptime2 <= uptime1 {
		t.Error("expected uptime to increase over time")
	}

	// Verify uptime is reasonable (less than a second for this test)
	if uptime2 > time.Second {
		t.Error("expected uptime to be less than a second for this test")
	}
}

func TestReadinessManager_ConcurrentAccess(t *testing.T) {
	rm := NewReadinessManager()
	done := make(chan bool)

	// Concurrent goroutines setting ready state
	for i := range 10 {
		go func(ready bool) {
			defer func() { done <- true }()
			rm.SetReady(ready)
		}(i%2 == 0)
	}

	// Concurrent goroutines reading ready state
	for range 10 {
		go func() {
			defer func() { done <- true }()
			_ = rm.IsReady()
		}()
	}

	// Concurrent goroutines reading uptime
	for range 10 {
		go func() {
			defer func() { done <- true }()
			_ = rm.GetUptime()
		}()
	}

	// Wait for all goroutines to complete
	for range 30 {
		<-done
	}

	// Verify manager is still functional
	rm.SetReady(true)
	if !rm.IsReady() {
		t.Error("expected ready state to be true after concurrent operations")
	}
}

func TestReadinessManager_AtomicOperations(t *testing.T) {
	rm := NewReadinessManager()

	// Test atomic swap behavior
	old := rm.ready.Swap(true)
	if old {
		t.Error("expected old value to be false")
	}

	if !rm.IsReady() {
		t.Error("expected ready state to be true after swap")
	}

	// Test swap again
	old = rm.ready.Swap(false)
	if !old {
		t.Error("expected old value to be true")
	}

	if rm.IsReady() {
		t.Error("expected ready state to be false after second swap")
	}
}

func TestReadinessManager_LoggerComponent(t *testing.T) {
	rm := NewReadinessManager()

	if rm.logger == nil {
		t.Fatal("expected logger to be initialized")
	}

	// Verify logger has correct component attribute by checking if it's not the default logger
	defaultLogger := slog.Default()
	if rm.logger == defaultLogger {
		t.Error("expected logger to be different from default logger (should have component attribute)")
	}
}

func BenchmarkReadinessManager_IsReady(b *testing.B) {
	rm := NewReadinessManager()
	rm.SetReady(true)

	b.ResetTimer()
	for b.Loop() {
		_ = rm.IsReady()
	}
}

func BenchmarkReadinessManager_GetUptime(b *testing.B) {
	rm := NewReadinessManager()

	b.ResetTimer()
	for b.Loop() {
		_ = rm.GetUptime()
	}
}

func BenchmarkReadinessManager_ConcurrentIsReady(b *testing.B) {
	rm := NewReadinessManager()
	rm.SetReady(true)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = rm.IsReady()
		}
	})
}
