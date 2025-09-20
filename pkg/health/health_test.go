package health

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewHealthManager(t *testing.T) {
	hm := NewHealthManager()

	if hm == nil {
		t.Fatal("expected HealthManager, got nil")
	}

	if hm.checkers == nil {
		t.Error("expected checkers map to be initialized")
	}

	if hm.ctx == nil {
		t.Error("expected context to be initialized")
	}

	if hm.cancel == nil {
		t.Error("expected cancel function to be initialized")
	}

	// Context should not be done initially
	select {
	case <-hm.ctx.Done():
		t.Error("expected context to not be done initially")
	default:
		// Expected
	}
}

func TestManager_RegisterChecker_ValidCases(t *testing.T) {
	tests := []struct {
		name        string
		checkerName string
		interval    time.Duration
		fn          CheckFunc
		expectErr   bool
	}{
		{
			name:        "valid registration",
			checkerName: "test-checker",
			interval:    1 * time.Second,
			fn:          func() error { return nil },
			expectErr:   false,
		},
		{
			name:        "minimum interval",
			checkerName: "min-checker",
			interval:    healthCheckMinInterval,
			fn:          func() error { return nil },
			expectErr:   false,
		},
		{
			name:        "below minimum interval gets adjusted",
			checkerName: "below-min-checker",
			interval:    500 * time.Millisecond,
			fn:          func() error { return nil },
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := NewHealthManager()
			defer hm.StopNoContext()

			err := hm.RegisterChecker(tt.checkerName, tt.interval, tt.fn)

			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			if !tt.expectErr {
				// Verify checker was registered
				if _, exists := hm.checkers[tt.checkerName]; !exists {
					t.Errorf("expected checker %q to be registered", tt.checkerName)
				}
			}
		})
	}
}

func TestManager_RegisterChecker_InvalidCases(t *testing.T) {
	tests := []struct {
		name        string
		checkerName string
		interval    time.Duration
		fn          CheckFunc
	}{
		{
			name:        "empty name",
			checkerName: "",
			interval:    1 * time.Second,
			fn:          func() error { return nil },
		},
		{
			name:        "whitespace only name",
			checkerName: "   ",
			interval:    1 * time.Second,
			fn:          func() error { return nil },
		},
		{
			name:        "nil function",
			checkerName: "test-checker",
			interval:    1 * time.Second,
			fn:          nil,
		},
		{
			name:        "zero interval",
			checkerName: "test-checker",
			interval:    0,
			fn:          func() error { return nil },
		},
		{
			name:        "negative interval",
			checkerName: "test-checker",
			interval:    -1 * time.Second,
			fn:          func() error { return nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hm := NewHealthManager()
			defer hm.StopNoContext()

			err := hm.RegisterChecker(tt.checkerName, tt.interval, tt.fn)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestManager_RegisterChecker_DuplicateName(t *testing.T) {
	hm := NewHealthManager()
	defer hm.StopNoContext()

	checkerName := "duplicate-checker"
	fn := func() error { return nil }
	interval := 1 * time.Second

	// Register first checker
	err := hm.RegisterChecker(checkerName, interval, fn)
	if err != nil {
		t.Fatalf("unexpected error registering first checker: %v", err)
	}

	// Try to register with same name
	err = hm.RegisterChecker(checkerName, interval, fn)
	if err == nil {
		t.Error("expected error for duplicate checker name, got nil")
	}
	if err.Error() != `checker "duplicate-checker" already exists` {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestManager_HealthStatus_NoCheckers(t *testing.T) {
	hm := NewHealthManager()
	defer hm.StopNoContext()

	results, anyFail := hm.HealthStatus()

	if results != nil {
		t.Error("expected nil results when no checkers are registered")
	}

	if anyFail {
		t.Error("expected anyFail to be false when no checkers are registered")
	}
}

func TestManager_HealthStatus_WithCheckers(t *testing.T) {
	hm := NewHealthManager()
	defer hm.StopNoContext()

	// Register checkers
	successChecker := func() error { return nil }
	failChecker := func() error { return errors.New("health check failed") }

	err := hm.RegisterChecker("success-checker", 1*time.Second, successChecker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = hm.RegisterChecker("fail-checker", 1*time.Second, failChecker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for checkers to run at least once
	time.Sleep(50 * time.Millisecond)

	results, anyFail := hm.HealthStatus()

	if results == nil {
		t.Fatal("expected results, got nil")
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	if results["success-checker"] != healthStatusOK {
		t.Errorf("expected success-checker status to be %q, got %q", healthStatusOK, results["success-checker"])
	}

	if results["fail-checker"] != healthStatusFail {
		t.Errorf("expected fail-checker status to be %q, got %q", healthStatusFail, results["fail-checker"])
	}

	if !anyFail {
		t.Error("expected anyFail to be true when at least one checker fails")
	}
}

func TestManager_HealthStatus_AllSuccess(t *testing.T) {
	hm := NewHealthManager()
	defer hm.StopNoContext()

	// Register only successful checkers
	successChecker := func() error { return nil }

	err := hm.RegisterChecker("success-1", 1*time.Second, successChecker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = hm.RegisterChecker("success-2", 1*time.Second, successChecker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for checkers to run
	time.Sleep(50 * time.Millisecond)

	results, anyFail := hm.HealthStatus()

	if anyFail {
		t.Error("expected anyFail to be false when all checkers succeed")
	}

	if results["success-1"] != healthStatusOK || results["success-2"] != healthStatusOK {
		t.Errorf("expected all checkers to have %q status, got %v", healthStatusOK, results)
	}
}

func TestManager_Stop(t *testing.T) {
	hm := NewHealthManager()

	// Register a checker
	err := hm.RegisterChecker("test-checker", 1*time.Second, func() error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stop should complete without error
	err = hm.StopNoContext()
	if err != nil {
		t.Errorf("unexpected error from Stop: %v", err)
	}

	// Context should be done after stop
	select {
	case <-hm.ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("expected context to be done after StopNoContext")
	}
}

func TestManager_StopWithoutCheckers(t *testing.T) {
	hm := NewHealthManager()

	// Stop without any checkers should work
	err := hm.StopNoContext()
	if err != nil {
		t.Errorf("unexpected error from StopNoContext: %v", err)
	}
}

func TestRegisteredChecker_Run_Success(t *testing.T) {
	checker := &registeredChecker{
		name: "test-checker",
		fn:   func() error { return nil },
	}

	checker.Run()

	lastErr := checker.lastError.Load()
	if lastErr != nil {
		t.Errorf("expected lastError to be nil for successful check, got %v", lastErr)
	}

	if checker.lastRun.IsZero() {
		t.Error("expected lastRun to be set")
	}

	// Verify lastRun is recent
	if time.Since(checker.lastRun) > time.Second {
		t.Error("expected lastRun to be recent")
	}
}

func TestRegisteredChecker_Run_Failure(t *testing.T) {
	testErr := errors.New("test error")
	checker := &registeredChecker{
		name: "test-checker",
		fn:   func() error { return testErr },
	}

	checker.Run()

	if checker.lastError.Load() == nil {
		t.Error("expected lastError to be set for failed check")
	}

	if loadedErr := checker.lastError.Load(); loadedErr == nil || *loadedErr != testErr {
		t.Errorf("expected lastError to be %v, got %v", testErr, loadedErr)
	}

	if checker.lastRun.IsZero() {
		t.Error("expected lastRun to be set even for failed check")
	}
}

func TestManager_ConcurrentRegisterAndStatus(t *testing.T) {
	hm := NewHealthManager()
	defer hm.StopNoContext()

	var wg sync.WaitGroup
	const numGoroutines = 10

	// Concurrent registrations
	for i := range numGoroutines {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			err := hm.RegisterChecker(
				fmt.Sprintf("checker-%d", index),
				1*time.Second,
				func() error { return nil },
			)
			if err != nil {
				t.Errorf("unexpected error registering checker: %v", err)
			}
		}(i)
	}

	// Concurrent status checks
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = hm.HealthStatus()
		}()
	}

	wg.Wait()

	// Verify all checkers were registered
	results, _ := hm.HealthStatus()
	if len(results) != numGoroutines {
		t.Errorf("expected %d checkers, got %d", numGoroutines, len(results))
	}
}

func TestManager_CheckerExecution(t *testing.T) {
	hm := NewHealthManager()
	defer hm.StopNoContext()

	var callCount int64
	checker := func() error {
		atomic.AddInt64(&callCount, 1)
		return nil
	}

	// Register with short interval
	interval := 50 * time.Millisecond
	err := hm.RegisterChecker("test-checker", interval, checker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for multiple executions (interval is 1s due to minimum)
	time.Sleep(2100 * time.Millisecond)

	count := atomic.LoadInt64(&callCount)
	if count < 2 {
		t.Errorf("expected at least 2 executions, got %d", count)
	}
}

func TestManager_CheckerExecutionAfterStop(t *testing.T) {
	hm := NewHealthManager()

	var callCount int64
	checker := func() error {
		atomic.AddInt64(&callCount, 1)
		return nil
	}

	err := hm.RegisterChecker("test-checker", 50*time.Millisecond, checker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait a bit for initial execution
	time.Sleep(100 * time.Millisecond)

	// Stop the manager
	err = hm.StopNoContext()
	if err != nil {
		t.Fatalf("unexpected error from StopNoContext: %v", err)
	}

	// Record call count after stop
	countAfterStop := atomic.LoadInt64(&callCount)

	// Wait and ensure no more calls happen
	time.Sleep(200 * time.Millisecond)
	finalCount := atomic.LoadInt64(&callCount)

	if finalCount != countAfterStop {
		t.Errorf("expected no more calls after stop, but count increased from %d to %d", countAfterStop, finalCount)
	}
}

func TestManager_IntervalAdjustment(t *testing.T) {
	hm := NewHealthManager()
	defer hm.StopNoContext()

	// Register checker with interval below minimum
	belowMinInterval := 100 * time.Millisecond // Below healthCheckMinInterval (1 second)
	err := hm.RegisterChecker("test-checker", belowMinInterval, func() error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The checker should be registered successfully, but internal interval should be adjusted
	if _, exists := hm.checkers["test-checker"]; !exists {
		t.Error("expected checker to be registered despite low interval")
	}
}

// Benchmark tests
func BenchmarkManager_RegisterChecker(b *testing.B) {
	hm := NewHealthManager()
	defer hm.StopNoContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := hm.RegisterChecker(
			fmt.Sprintf("checker-%d", i),
			1*time.Second,
			func() error { return nil },
		)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkManager_HealthStatus(b *testing.B) {
	hm := NewHealthManager()
	defer hm.StopNoContext()

	// Register some checkers
	for i := range 10 {
		err := hm.RegisterChecker(
			fmt.Sprintf("checker-%d", i),
			1*time.Second,
			func() error { return nil },
		)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = hm.HealthStatus()
	}
}

func BenchmarkRegisteredChecker_Run(b *testing.B) {
	checker := &registeredChecker{
		name: "bench-checker",
		fn:   func() error { return nil },
	}

	b.ResetTimer()
	for b.Loop() {
		checker.Run()
	}
}

func BenchmarkManager_ConcurrentHealthStatus(b *testing.B) {
	hm := NewHealthManager()
	defer hm.StopNoContext()

	// Register some checkers
	for i := range 10 {
		err := hm.RegisterChecker(
			fmt.Sprintf("checker-%d", i),
			1*time.Second,
			func() error { return nil },
		)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = hm.HealthStatus()
		}
	})
}

// Additional edge case tests
func TestManager_EdgeCases(t *testing.T) {
	t.Run("context already canceled", func(t *testing.T) {
		hm := NewHealthManager()
		hm.cancel() // Cancel context immediately

		// Registering checker should still work
		err := hm.RegisterChecker("test-checker", 1*time.Second, func() error { return nil })
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// But checker should not run for long
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("very large number of checkers", func(t *testing.T) {
		hm := NewHealthManager()
		defer hm.StopNoContext()

		const numCheckers = 1000
		for i := 0; i < numCheckers; i++ {
			err := hm.RegisterChecker(
				fmt.Sprintf("stress-checker-%d", i),
				1*time.Second,
				func() error { return nil },
			)
			if err != nil {
				t.Errorf("failed to register checker %d: %v", i, err)
				break
			}
		}

		// Verify all checkers are registered
		results, _ := hm.HealthStatus()
		if len(results) != numCheckers {
			t.Errorf("expected %d checkers, got %d", numCheckers, len(results))
		}
	})

	t.Run("checker with error", func(t *testing.T) {
		hm := NewHealthManager()
		defer hm.StopNoContext()

		// Use an error instead of panic for testing
		errorChecker := func() error {
			return errors.New("test error")
		}

		err := hm.RegisterChecker("error-checker", 1*time.Second, errorChecker)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Let the checker run and verify it fails gracefully
		time.Sleep(50 * time.Millisecond)

		results, anyFail := hm.HealthStatus()
		if !anyFail {
			t.Error("expected health check to fail")
		}
		if results["error-checker"] != healthStatusFail {
			t.Errorf("expected error-checker to fail, got %v", results["error-checker"])
		}
	})

	t.Run("empty name variations", func(t *testing.T) {
		hm := NewHealthManager()
		defer hm.StopNoContext()

		testCases := []string{
			"",
			" ",
			"\t",
			"\n",
			"   \t\n   ",
		}

		for _, name := range testCases {
			err := hm.RegisterChecker(name, 1*time.Second, func() error { return nil })
			if err == nil {
				t.Errorf("expected error for name %q, got nil", name)
			}
		}
	})
}

func TestManager_MemoryLeaks(t *testing.T) {
	t.Run("repeated start stop cycles", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			hm := NewHealthManager()
			err := hm.RegisterChecker("test-checker", 1*time.Second, func() error { return nil })
			if err != nil {
				t.Errorf("failed to register checker on iteration %d: %v", i, err)
			}
			err = hm.StopNoContext()
			if err != nil {
				t.Errorf("failed to stop on iteration %d: %v", i, err)
			}
		}
	})

	t.Run("goroutine cleanup verification", func(t *testing.T) {
		runtime.GC() // Force garbage collection before test
		initialGoroutines := runtime.NumGoroutine()

		hm := NewHealthManager()
		for i := 0; i < 10; i++ {
			err := hm.RegisterChecker(
				fmt.Sprintf("cleanup-checker-%d", i),
				100*time.Millisecond,
				func() error { return nil },
			)
			if err != nil {
				t.Fatalf("failed to register checker: %v", err)
			}
		}

		// Let checkers run
		time.Sleep(200 * time.Millisecond)

		// Stop and verify cleanup
		err := hm.StopNoContext()
		if err != nil {
			t.Fatalf("failed to stop: %v", err)
		}

		// Give time for cleanup
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
		time.Sleep(100 * time.Millisecond)

		finalGoroutines := runtime.NumGoroutine()
		// Allow some tolerance for test framework goroutines
		if finalGoroutines > initialGoroutines+5 {
			t.Errorf("potential goroutine leak: initial=%d, final=%d", initialGoroutines, finalGoroutines)
		}
	})
}
