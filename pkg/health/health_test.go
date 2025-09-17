package health

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewHealthManager(t *testing.T) {
	t.Parallel()

	manager := NewHealthManager()
	if manager == nil {
		t.Fatal("NewHealthManager returned nil")
	}

	if manager.checkers == nil {
		t.Error("checkers map not initialized")
	}

	if manager.ctx == nil {
		t.Error("context not initialized")
	}

	if manager.cancel == nil {
		t.Error("cancel function not initialized")
	}

	if manager.logger == nil {
		t.Error("logger not initialized")
	}
}

func TestManager_RegisterChecker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		checkerName string
		interval    time.Duration
		fn          CheckFunc
		wantErr     bool
	}{
		{
			name:        "valid checker",
			checkerName: "test-checker",
			interval:    5 * time.Second,
			fn: func(_ context.Context) error {
				return nil
			},
			wantErr: false,
		},
		{
			name:        "empty name",
			checkerName: "",
			interval:    5 * time.Second,
			fn: func(_ context.Context) error {
				return nil
			},
			wantErr: true,
		},
		{
			name:        "whitespace name",
			checkerName: "   ",
			interval:    5 * time.Second,
			fn: func(_ context.Context) error {
				return nil
			},
			wantErr: true,
		},
		{
			name:        "nil function",
			checkerName: "test-checker",
			interval:    5 * time.Second,
			fn:          nil,
			wantErr:     true,
		},
		{
			name:        "zero interval",
			checkerName: "test-checker",
			interval:    0,
			fn: func(_ context.Context) error {
				return nil
			},
			wantErr: true,
		},
		{
			name:        "negative interval",
			checkerName: "test-checker",
			interval:    -5 * time.Second,
			fn: func(_ context.Context) error {
				return nil
			},
			wantErr: true,
		},
		{
			name:        "interval below minimum",
			checkerName: "test-checker",
			interval:    500 * time.Millisecond,
			fn: func(_ context.Context) error {
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			manager := NewHealthManager()
			defer func() {
				_ = manager.Stop(context.Background())
			}()

			err := manager.RegisterChecker(tt.checkerName, tt.interval, tt.fn)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterChecker() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				manager.mu.RLock()
				_, exists := manager.checkers[tt.checkerName]
				manager.mu.RUnlock()
				if !exists {
					t.Error("checker was not registered")
				}
			}
		})
	}
}

func TestManager_RegisterChecker_Duplicate(t *testing.T) {
	t.Parallel()

	manager := NewHealthManager()
	defer func() {
		_ = manager.Stop(context.Background())
	}()

	checkerName := "duplicate-checker"
	fn := func(_ context.Context) error {
		return nil
	}

	// Register first time - should succeed
	err := manager.RegisterChecker(checkerName, 5*time.Second, fn)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Register second time - should fail
	err = manager.RegisterChecker(checkerName, 5*time.Second, fn)
	if err == nil {
		t.Error("expected error for duplicate checker registration")
	}
}

func TestManager_GetHealthStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupCheckers  func(*Manager)
		expectedStatus map[string]string
		expectedFail   bool
	}{
		{
			name:           "no checkers",
			setupCheckers:  func(_ *Manager) {},
			expectedStatus: nil,
			expectedFail:   false,
		},
		{
			name: "single passing checker",
			setupCheckers: func(m *Manager) {
				err := m.RegisterChecker("pass", 100*time.Millisecond, func(_ context.Context) error {
					return nil
				})
				if err != nil {
					t.Fatalf("failed to register checker: %v", err)
				}
				// Wait for at least one execution
				time.Sleep(150 * time.Millisecond)
			},
			expectedStatus: map[string]string{"pass": "ok"},
			expectedFail:   false,
		},
		{
			name: "single failing checker",
			setupCheckers: func(m *Manager) {
				err := m.RegisterChecker("fail", 100*time.Millisecond, func(_ context.Context) error {
					return errors.New("test error")
				})
				if err != nil {
					t.Fatalf("failed to register checker: %v", err)
				}
				// Wait for at least one execution
				time.Sleep(150 * time.Millisecond)
			},
			expectedStatus: map[string]string{"fail": "fail"},
			expectedFail:   true,
		},
		{
			name: "mixed checkers",
			setupCheckers: func(m *Manager) {
				err := m.RegisterChecker("pass", 100*time.Millisecond, func(_ context.Context) error {
					return nil
				})
				if err != nil {
					t.Fatalf("failed to register pass checker: %v", err)
				}
				err = m.RegisterChecker("fail", 100*time.Millisecond, func(_ context.Context) error {
					return errors.New("test error")
				})
				if err != nil {
					t.Fatalf("failed to register fail checker: %v", err)
				}
				// Wait for at least one execution
				time.Sleep(150 * time.Millisecond)
			},
			expectedStatus: map[string]string{"pass": "ok", "fail": "fail"},
			expectedFail:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			manager := NewHealthManager()
			defer func() {
				_ = manager.Stop(context.Background())
			}()

			tt.setupCheckers(manager)

			status, anyFail := manager.GetHealthStatus()

			if tt.expectedStatus == nil && status != nil {
				t.Errorf("expected nil status, got %v", status)
			}

			if tt.expectedStatus != nil {
				if status == nil {
					t.Error("expected non-nil status")
				} else {
					for expectedName, expectedState := range tt.expectedStatus {
						if actualState, exists := status[expectedName]; !exists {
							t.Errorf("expected checker %q not found", expectedName)
						} else if actualState != expectedState {
							t.Errorf("checker %q: expected state %q, got %q", expectedName, expectedState, actualState)
						}
					}
				}
			}

			if anyFail != tt.expectedFail {
				t.Errorf("expected anyFail %v, got %v", tt.expectedFail, anyFail)
			}
		})
	}
}

func TestManager_Stop(t *testing.T) {
	t.Parallel()

	t.Run("normal shutdown", func(t *testing.T) {
		t.Parallel()
		manager := NewHealthManager()

		// Register a checker
		err := manager.RegisterChecker("test", 1*time.Second, func(_ context.Context) error {
			return nil
		})
		if err != nil {
			t.Fatalf("failed to register checker: %v", err)
		}

		// Stop the manager
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = manager.Stop(ctx)
		if err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})

	t.Run("timeout shutdown", func(t *testing.T) {
		t.Parallel()
		manager := NewHealthManager()

		// Register a checker that blocks
		err := manager.RegisterChecker("blocking", 1*time.Second, func(ctx context.Context) error {
			// This will block until context is cancelled
			<-ctx.Done()
			// Simulate a slow shutdown
			time.Sleep(100 * time.Millisecond)
			return nil
		})
		if err != nil {
			t.Fatalf("failed to register checker: %v", err)
		}

		// Stop with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		err = manager.Stop(ctx)
		if err == nil {
			t.Error("expected timeout error")
		}
	})
}

func TestManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	manager := NewHealthManager()
	defer func() {
		_ = manager.Stop(context.Background())
	}()

	const numGoroutines = 10
	const numCheckers = 10

	var wg sync.WaitGroup

	// Concurrently register checkers
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range numCheckers {
				checkerName := fmt.Sprintf("checker-%d-%d", id, j)
				err := manager.RegisterChecker(checkerName, 1*time.Second, func(_ context.Context) error {
					return nil
				})
				if err != nil {
					t.Errorf("failed to register checker %s: %v", checkerName, err)
				}
			}
		}(i)
	}

	// Concurrently read health status
	for i := range numGoroutines {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			for range 10 {
				_, _ = manager.GetHealthStatus()
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Verify all checkers were registered
	status, _ := manager.GetHealthStatus()
	expectedCount := numGoroutines * numCheckers
	if len(status) != expectedCount {
		t.Errorf("expected %d checkers, got %d", expectedCount, len(status))
	}
}

func TestManager_CheckerExecution(t *testing.T) {
	t.Parallel()

	t.Run("checker receives context cancellation", func(t *testing.T) {
		t.Parallel()
		manager := NewHealthManager()

		var contextCancelled bool
		var mu sync.Mutex

		err := manager.RegisterChecker("context-test", 50*time.Millisecond, func(ctx context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			if ctx.Err() != nil {
				contextCancelled = true
			}
			return nil
		})
		if err != nil {
			t.Fatalf("failed to register checker: %v", err)
		}

		// Let it run a few times
		time.Sleep(200 * time.Millisecond)

		// Stop the manager
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = manager.Stop(ctx)
		if err != nil {
			t.Errorf("Stop() error = %v", err)
		}

		// Check if context cancellation was observed
		mu.Lock()
		defer mu.Unlock()
		if !contextCancelled {
			t.Error("checker should have observed context cancellation")
		}
	})

	t.Run("failed checker updates state", func(t *testing.T) {
		t.Parallel()
		manager := NewHealthManager()
		defer func() {
			_ = manager.Stop(context.Background())
		}()

		failCount := 0
		err := manager.RegisterChecker("fail-test", 50*time.Millisecond, func(_ context.Context) error {
			failCount++
			if failCount <= 2 {
				return errors.New("intentional failure")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("failed to register checker: %v", err)
		}

		// Wait for multiple executions - note that interval gets adjusted to minimum (1s)
		// so we need to wait longer for recovery
		time.Sleep(2500 * time.Millisecond)

		status, anyFail := manager.GetHealthStatus()
		if status["fail-test"] != "ok" {
			t.Errorf("expected checker to recover, got state: %s", status["fail-test"])
		}
		if anyFail {
			t.Error("expected no failures after recovery")
		}
	})
}
