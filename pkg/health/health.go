package health

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	healthCheckDebugLogInterval = 5 * time.Minute
	consecutiveFailureThreshold = 3
	healthCheckMinInterval      = 1 * time.Second

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
	logger   *slog.Logger
}

// NewHealthManager creates a new HealthManager.
func NewHealthManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		checkers: make(map[string]*registeredChecker),
		ctx:      ctx,
		cancel:   cancel,
		logger:   slog.Default().With("component", "health.manager"),
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
		h.logger.Warn("checker interval is less than the minimum interval, using minimum interval",
			"name", name,
			"requested_interval", requestedInterval.String(),
			"minimum_interval", healthCheckMinInterval.String())
		interval = healthCheckMinInterval
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.checkers[name]; exists {
		return fmt.Errorf("checker %q already exists", name)
	}

	rc := &registeredChecker{fn: fn, interval: interval, lastState: "unknown"}
	h.checkers[name] = rc

	h.wg.Go(func() { h.runChecker(name, rc) })

	h.logger.Info("registered health checker",
		"name", name,
		"interval", interval.String(),
		"requested_interval", requestedInterval.String())
	return nil
}

// GetHealthStatus returns the latest state of all registered checkers.
func (h *Manager) GetHealthStatus() (map[string]string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.checkers) == 0 {
		return nil, false
	}

	results := make(map[string]string, len(h.checkers))
	anyFail := false

	for name, c := range h.checkers {
		state := "unknown"
		if c == nil {
			state = "skipped"
		} else if c.lastState != "" {
			state = c.lastState
		}

		results[name] = state
		if state == healthStatusFail {
			anyFail = true
		}
	}

	return results, anyFail
}

// Stop stops all health checkers.
func (h *Manager) Stop(ctx context.Context) error {
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
		return nil
	case <-ctx.Done():
		return fmt.Errorf("health manager shutdown timeout: %w", ctx.Err())
	}
}

// runChecker executes one checker periodically until context is canceled.
func (h *Manager) runChecker(name string, c *registeredChecker) {
	// run once immediately
	h.logger.Debug("checker tick", "name", name, "phase", "initial")
	h.executeChecker(name, c)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			// Final tick on cancellation:
			// Give the checker one last chance to observe context cancellation and
			// update its state before shutdown. This happens at most once.
			if c != nil && !c.cancelSeen {
				h.logger.Debug("checker tick", "name", name, "phase", "final")
				h.executeChecker(name, c)
			}
			return
		case <-ticker.C:
			h.logger.Debug("checker tick", "name", name, "phase", "periodic")
			h.executeChecker(name, c)
		}
	}
}

// executeChecker runs a single checker and updates its last state.
func (h *Manager) executeChecker(name string, c *registeredChecker) {
	if c == nil || c.fn == nil {
		h.markCheckerSkipped(name)
		return
	}

	err := c.fn(h.ctx)
	result := h.updateCheckerState(name, err)
	h.logCheckerResult(name, c, result)
}

// CheckFunc defines a checker function that returns error when unhealthy.
type CheckFunc func(ctx context.Context) error

// registeredChecker holds checker settings and last result.
type registeredChecker struct {
	fn               CheckFunc
	interval         time.Duration
	lastState        string
	lastRun          time.Time
	cancelSeen       bool
	lastLogTime      time.Time // for rate limiting debug logs
	consecutiveFails int       // track consecutive failures for better logging
}

// checkerResult holds the result of a checker execution.
type checkerResult struct {
	prevState        string
	newState         string
	prevLastRun      time.Time
	now              time.Time
	consecutiveFails int
	shouldLogDebug   bool
	err              error
}

// markCheckerSkipped marks a checker as skipped.
func (h *Manager) markCheckerSkipped(name string) {
	h.mu.Lock()
	if existing, ok := h.checkers[name]; ok && existing != nil {
		existing.lastState = "skipped"
		existing.lastRun = time.Now()
	}
	h.mu.Unlock()
	h.logger.Debug("checker skipped", "name", name)
}

// updateCheckerState updates the checker state and returns the result.
func (h *Manager) updateCheckerState(name string, err error) *checkerResult {
	now := time.Now()
	result := &checkerResult{now: now, err: err}

	h.mu.Lock()
	defer h.mu.Unlock()

	existing, ok := h.checkers[name]
	if !ok || existing == nil {
		return result
	}

	result.prevState = existing.lastState
	result.prevLastRun = existing.lastRun

	if err != nil {
		result.newState = healthStatusFail
		existing.consecutiveFails++
	} else {
		result.newState = healthStatusOK
		existing.consecutiveFails = 0
	}

	existing.lastState = result.newState
	if h.ctx.Err() != nil {
		existing.cancelSeen = true
	}
	existing.lastRun = now
	result.consecutiveFails = existing.consecutiveFails

	// Rate limiting for debug logs
	result.shouldLogDebug = now.Sub(existing.lastLogTime) >= healthCheckDebugLogInterval
	if result.shouldLogDebug && (result.prevState == result.newState) {
		existing.lastLogTime = now
	}

	return result
}

// logCheckerResult logs the checker execution result.
func (h *Manager) logCheckerResult(name string, c *registeredChecker, result *checkerResult) {
	if result.newState == "" {
		return
	}

	baseFields := h.buildLogFields(name, c, result)
	if result.prevState != result.newState {
		h.logStateTransition(result, baseFields)
	} else {
		h.logSteadyState(result, baseFields)
	}
}

// buildLogFields constructs common log fields for checker results.
func (h *Manager) buildLogFields(name string, c *registeredChecker, result *checkerResult) []any {
	duration := time.Duration(0)
	if !result.prevLastRun.IsZero() {
		duration = result.now.Sub(result.prevLastRun)
	}

	return []any{
		"name", name,
		"interval", c.interval.String(),
		"duration", duration.String(),
	}
}

// logStateTransition logs health check state changes.
func (h *Manager) logStateTransition(result *checkerResult, baseFields []any) {
	if result.newState == healthStatusFail {
		h.logger.Warn(
			"health check state changed to failing",
			append(
				baseFields,
				"from",
				result.prevState,
				"to",
				result.newState,
				"consecutive_failures",
				result.consecutiveFails,
				"error",
				result.err.Error(),
			)...)
	} else {
		h.logger.Info("health check recovered",
			append(baseFields, "from", result.prevState, "to", result.newState, "previous_failures", result.consecutiveFails)...)
	}
}

// logSteadyState logs health check steady state with rate limiting.
func (h *Manager) logSteadyState(result *checkerResult, baseFields []any) {
	if !result.shouldLogDebug {
		return
	}

	if result.newState == healthStatusFail {
		if result.consecutiveFails >= consecutiveFailureThreshold {
			h.logger.Warn("health check persistently failing",
				append(baseFields, "consecutive_failures", result.consecutiveFails, "error", result.err.Error())...)
		} else {
			h.logger.Debug("health check failing",
				append(baseFields, "consecutive_failures", result.consecutiveFails, "error", result.err.Error())...)
		}
	} else {
		h.logger.Debug("health check passing", baseFields...)
	}
}
