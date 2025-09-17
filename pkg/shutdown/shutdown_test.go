package shutdown

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"syscall"
	"testing"
	"time"
)

type mockShutdowner struct {
	shutdownFunc func(ctx context.Context) error
	shutdownTime time.Duration
}

func (m *mockShutdowner) Shutdown(ctx context.Context) error {
	if m.shutdownTime > 0 {
		time.Sleep(m.shutdownTime)
	}
	if m.shutdownFunc != nil {
		return m.shutdownFunc(ctx)
	}
	return nil
}

func TestFunc_Shutdown(t *testing.T) {
	called := false
	f := Func(func(_ context.Context) error {
		called = true
		return nil
	})

	err := f.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Func.Shutdown() error = %v", err)
	}

	if !called {
		t.Error("Func.Shutdown() did not call the function")
	}
}

func TestNewManager(t *testing.T) {
	manager := NewManager()

	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}

	if len(manager.shutdowners) != 0 {
		t.Errorf("expected 0 shutdowners, got %d", len(manager.shutdowners))
	}

	if manager.timeout != DefaultShutdownTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultShutdownTimeout, manager.timeout)
	}

	expectedSignals := []os.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT}
	if len(manager.signals) != len(expectedSignals) {
		t.Errorf("expected %d signals, got %d", len(expectedSignals), len(manager.signals))
	}
}

func TestManager_WithTimeout(t *testing.T) {
	manager := NewManager()

	// Test valid timeout
	newTimeout := 10 * time.Second
	result := manager.WithTimeout(newTimeout)

	if result != manager {
		t.Error("WithTimeout() should return the same manager instance")
	}

	if manager.timeout != newTimeout {
		t.Errorf("expected timeout %v, got %v", newTimeout, manager.timeout)
	}

	// Test invalid timeout (should be ignored)
	originalTimeout := manager.timeout
	manager.WithTimeout(0)

	if manager.timeout != originalTimeout {
		t.Error("zero timeout should be ignored")
	}

	manager.WithTimeout(-5 * time.Second)
	if manager.timeout != originalTimeout {
		t.Error("negative timeout should be ignored")
	}
}

func TestManager_WithSignals(t *testing.T) {
	manager := NewManager()

	// Test valid signals
	newSignals := []os.Signal{syscall.SIGUSR1, syscall.SIGUSR2}
	result := manager.WithSignals(newSignals...)

	if result != manager {
		t.Error("WithSignals() should return the same manager instance")
	}

	if len(manager.signals) != len(newSignals) {
		t.Errorf("expected %d signals, got %d", len(newSignals), len(manager.signals))
	}

	// Test empty signals (should be ignored)
	originalSignals := manager.signals
	manager.WithSignals()

	if len(manager.signals) != len(originalSignals) {
		t.Error("empty signals should be ignored")
	}
}

func TestManager_WithLogger(t *testing.T) {
	manager := NewManager()

	// Test with valid logger
	logger := slog.Default()
	result := manager.WithLogger(logger)

	if result != manager {
		t.Error("WithLogger() should return the same manager instance")
	}

	// Test with nil logger (should use default)
	manager.WithLogger(nil)
	if manager.logger == nil {
		t.Error("WithLogger(nil) should set a default logger")
	}
}

func TestManager_Register(t *testing.T) {
	manager := NewManager()

	// Test registering valid shutdowner
	shutdowner := &mockShutdowner{}
	manager.Register(shutdowner)

	if len(manager.shutdowners) != 1 {
		t.Errorf("expected 1 shutdowner, got %d", len(manager.shutdowners))
	}

	// Test registering nil shutdowner (should be ignored)
	manager.Register(nil)

	if len(manager.shutdowners) != 1 {
		t.Error("nil shutdowner should be ignored")
	}
}

func TestManager_RegisterFunc(t *testing.T) {
	manager := NewManager()

	// Test registering valid function
	fn := func(_ context.Context) error {
		return nil
	}
	manager.RegisterFunc(fn)

	if len(manager.shutdowners) != 1 {
		t.Errorf("expected 1 shutdowner, got %d", len(manager.shutdowners))
	}

	// Test registering nil function (should be ignored)
	manager.RegisterFunc(nil)

	if len(manager.shutdowners) != 1 {
		t.Error("nil function should be ignored")
	}
}

func TestManager_Shutdown(t *testing.T) {
	tests := []struct {
		name        string
		shutdowners []Shutdowner
		timeout     time.Duration
		wantErr     bool
	}{
		{
			name:        "no shutdowners",
			shutdowners: []Shutdowner{},
			timeout:     1 * time.Second,
			wantErr:     false,
		},
		{
			name: "successful shutdown",
			shutdowners: []Shutdowner{
				&mockShutdowner{},
				&mockShutdowner{},
			},
			timeout: 1 * time.Second,
			wantErr: false,
		},
		{
			name: "shutdown with error",
			shutdowners: []Shutdowner{
				&mockShutdowner{
					shutdownFunc: func(_ context.Context) error {
						return errors.New("shutdown error")
					},
				},
			},
			timeout: 1 * time.Second,
			wantErr: true,
		},
		{
			name: "shutdown timeout",
			shutdowners: []Shutdowner{
				&mockShutdowner{
					shutdownTime: 200 * time.Millisecond,
				},
			},
			timeout: 100 * time.Millisecond,
			wantErr: true,
		},
		{
			name: "partial success with errors",
			shutdowners: []Shutdowner{
				&mockShutdowner{}, // successful
				&mockShutdowner{
					shutdownFunc: func(_ context.Context) error {
						return errors.New("first error")
					},
				},
				&mockShutdowner{}, // successful
				&mockShutdowner{
					shutdownFunc: func(_ context.Context) error {
						return errors.New("second error")
					},
				},
			},
			timeout: 1 * time.Second,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager().WithTimeout(tt.timeout)

			for _, s := range tt.shutdowners {
				manager.Register(s)
			}

			err := manager.Shutdown()
			if (err != nil) != tt.wantErr {
				t.Errorf("Manager.Shutdown() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Test that calling Shutdown again works (once.Do behavior)
			// Note: Due to once.Do, the second call may return nil even if first failed
			_ = manager.Shutdown()
		})
	}
}

func TestManager_WaitWithContext(t *testing.T) {
	// Test context cancellation
	manager := NewManager().WithTimeout(1 * time.Second)
	shutdowner := &mockShutdowner{}
	manager.Register(shutdowner)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately to trigger shutdown
	cancel()

	err := manager.WaitWithContext(ctx)
	if err != nil {
		t.Errorf("WaitWithContext() error = %v", err)
	}
}

func TestListenAndShutdown(_ *testing.T) {
	shutdowner1 := &mockShutdowner{}
	shutdowner2 := &mockShutdowner{}

	// This test will timeout quickly since we're not sending any signals
	done := make(chan error, 1)
	go func() {
		done <- ListenAndShutdown(shutdowner1, shutdowner2)
	}()

	// Give it a brief moment to set up, then send signal via context cancel
	time.Sleep(10 * time.Millisecond)

	// We can't easily test signal handling in unit tests, so just verify
	// the function doesn't panic and accepts the shutdowners
	select {
	case <-done:
		// Function returned, which is unexpected without signal but ok for test
	case <-time.After(100 * time.Millisecond):
		// Expected - function is waiting for signal
	}
}

func TestListenAndShutdownWithTimeout(_ *testing.T) {
	shutdowner := &mockShutdowner{}
	timeout := 500 * time.Millisecond

	// This test will timeout quickly since we're not sending any signals
	done := make(chan error, 1)
	go func() {
		done <- ListenAndShutdownWithTimeout(timeout, shutdowner)
	}()

	// Give it a brief moment to set up
	time.Sleep(10 * time.Millisecond)

	// We can't easily test signal handling in unit tests, so just verify
	// the function doesn't panic and accepts the parameters
	select {
	case <-done:
		// Function returned, which is unexpected without signal but ok for test
	case <-time.After(100 * time.Millisecond):
		// Expected - function is waiting for signal
	}
}

func TestManager_ConcurrentShutdown(t *testing.T) {
	manager := NewManager().WithTimeout(1 * time.Second)

	// Add multiple shutdowners with different behaviors
	for i := range 5 {
		manager.Register(&mockShutdowner{
			shutdownTime: time.Duration(i*10) * time.Millisecond,
		})
	}

	// Call shutdown multiple times concurrently
	results := make(chan error, 10)
	for range 10 {
		go func() {
			results <- manager.Shutdown()
		}()
	}

	// Collect results
	var firstErr error
	for range 10 {
		err := <-results
		if firstErr == nil {
			firstErr = err
		} else if !errors.Is(err, firstErr) {
			t.Error("concurrent calls to Shutdown should return the same error")
		}
	}
}
