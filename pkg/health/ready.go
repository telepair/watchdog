package health

import (
	"log/slog"
	"sync/atomic"
	"time"
)

// ReadinessManager manages readiness state.
type ReadinessManager struct {
	ready     atomic.Bool
	startTime time.Time
}

// NewReadinessManager creates a new ReadinessManager.
func NewReadinessManager() *ReadinessManager {
	rm := &ReadinessManager{
		startTime: time.Now(),
	}
	rm.ready.Store(false)
	return rm
}

// SetReady sets the ready state.
func (r *ReadinessManager) SetReady(ready bool) {
	old := r.ready.Swap(ready)

	if old != ready {
		slog.Info("readiness state changed", "from", old, "to", ready)
	} else {
		slog.Debug("readiness state unchanged", "ready", ready)
	}
}

// IsReady returns the current ready state.
func (r *ReadinessManager) IsReady() bool {
	return r.ready.Load()
}

// GetUptime returns the time since start.
func (r *ReadinessManager) GetUptime() time.Duration {
	return time.Since(r.startTime)
}
