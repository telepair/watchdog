package health

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	healthCheckMinInterval = 1 * time.Second
	shutdownTimeout        = 5 * time.Second

	// Health check status constants.
	healthStatusOK   = "ok"
	healthStatusFail = "fail"
)

// Manager manages health checks.
type Manager struct {
	checkers map[string]*registeredChecker
	mu       sync.RWMutex
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewHealthManager creates a new HealthManager.
func NewHealthManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		checkers: make(map[string]*registeredChecker),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// RegisterChecker registers a named health checker with a given interval.
func (h *Manager) RegisterChecker(name string, interval time.Duration, fn CheckFunc) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("checker name is required")
	}
	if fn == nil {
		return errors.New("checker func is required")
	}
	if interval <= 0 {
		return errors.New("checker interval must be greater than zero")
	}
	requestedInterval := interval
	if interval < healthCheckMinInterval {
		slog.Warn("health checker interval is less than the minimum interval, using minimum interval",
			"name", name,
			"requested_interval", requestedInterval.String(),
			"minimum_interval", healthCheckMinInterval.String())
		interval = healthCheckMinInterval
	}

	h.mu.Lock()
	if _, exists := h.checkers[name]; exists {
		h.mu.Unlock()
		slog.Error("health checker already exists", "name", name)
		return fmt.Errorf("checker %q already exists", name)
	}

	rc := &registeredChecker{name: name, fn: fn}
	h.checkers[name] = rc
	h.mu.Unlock()

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		h.runChecker(name, interval, rc)
	}()

	slog.Info("registered health checker",
		"name", name,
		"interval", interval.String(),
		"requested_interval", requestedInterval.String())
	return nil
}

// GetHealthStatus returns the latest state of all registered checkers.
func (h *Manager) GetHealthStatus() (map[string]string, bool) {
	return h.HealthStatus()
}

// HealthStatus returns the latest state of all registered checkers.
func (h *Manager) HealthStatus() (map[string]string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.checkers) == 0 {
		return nil, false
	}

	results := make(map[string]string, len(h.checkers))
	anyFail := false

	for name, c := range h.checkers {
		results[name] = healthStatusOK
		if lastErr := c.lastError.Load(); lastErr != nil {
			results[name] = healthStatusFail
			anyFail = true
		}
	}

	return results, anyFail
}

// Stop stops all health checkers.
func (h *Manager) Stop(ctx context.Context) error {
	_ = ctx // context is passed but not used in this implementation
	return h.StopNoContext()
}

// StopNoContext stops all health checkers without context.
func (h *Manager) StopNoContext() error {
	slog.Info("stopping health manager")
	if h.cancel != nil {
		h.cancel()
	}

	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("health manager stopped")
		return nil
	case <-time.After(shutdownTimeout):
		return fmt.Errorf("health manager shutdown timeout")
	}
}

// runChecker executes one checker periodically until context is canceled.
func (h *Manager) runChecker(name string, interval time.Duration, c *registeredChecker) {
	if c == nil {
		return
	}

	slog.Debug("health checker loop started", "name", name)
	c.Run()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			slog.Debug("health checker loop stopped", "name", name)
			return
		case <-ticker.C:
			c.Run()
		}
	}
}

// CheckFunc defines a checker function that returns error when unhealthy.
type CheckFunc func() error

// registeredChecker holds checker settings and last result.
type registeredChecker struct {
	name      string
	fn        CheckFunc
	lastError atomic.Pointer[error]
	lastRun   time.Time
}

func (r *registeredChecker) Run() {
	slog.Debug("running health checker", "name", r.name)
	if err := r.fn(); err != nil {
		r.lastError.Store(&err)
		slog.Error("health checker failed", "name", r.name, "error", err)
	} else {
		slog.Debug("health checker passed", "name", r.name)
		r.lastError.Store(nil)
	}
	r.lastRun = time.Now()
}
