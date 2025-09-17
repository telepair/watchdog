package shutdown

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	// DefaultShutdownTimeout is the default timeout for graceful shutdown.
	DefaultShutdownTimeout = 5 * time.Second
)

// Shutdowner represents any component that can be gracefully shutdown.
type Shutdowner interface {
	Shutdown(ctx context.Context) error
}

// Func is a function that performs shutdown operations.
type Func func(ctx context.Context) error

// Shutdown implements Shutdowner interface.
func (f Func) Shutdown(ctx context.Context) error {
	return f(ctx)
}

// Manager manages graceful shutdown of multiple components.
type Manager struct {
	shutdowners []Shutdowner
	signals     []os.Signal
	timeout     time.Duration
	logger      *slog.Logger
	once        sync.Once
}

// NewManager creates a new shutdown manager with default configuration.
func NewManager() *Manager {
	return &Manager{
		shutdowners: make([]Shutdowner, 0),
		signals:     []os.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT},
		timeout:     DefaultShutdownTimeout,
		logger:      slog.Default().With("component", "shutdown.manager"),
	}
}

// WithTimeout sets the shutdown timeout.
func (m *Manager) WithTimeout(timeout time.Duration) *Manager {
	if timeout > 0 {
		m.timeout = timeout
	}
	return m
}

// WithSignals sets the signals to listen for.
func (m *Manager) WithSignals(signals ...os.Signal) *Manager {
	if len(signals) > 0 {
		m.signals = signals
	}
	return m
}

// WithLogger sets the logger.
func (m *Manager) WithLogger(logger *slog.Logger) *Manager {
	if logger != nil {
		m.logger = logger.With("component", "shutdown.manager")
	}
	return m
}

// Register registers a shutdowner component.
func (m *Manager) Register(shutdowner Shutdowner) {
	if shutdowner != nil {
		m.shutdowners = append(m.shutdowners, shutdowner)
	}
}

// RegisterFunc registers a shutdown function.
func (m *Manager) RegisterFunc(fn func(ctx context.Context) error) {
	if fn != nil {
		m.Register(Func(fn))
	}
}

// Wait blocks until a shutdown signal is received, then performs graceful shutdown.
func (m *Manager) Wait() error {
	return m.WaitWithContext(context.Background())
}

// WaitWithContext blocks until a shutdown signal is received or context is cancelled.
func (m *Manager) WaitWithContext(ctx context.Context) error {
	// Buffer size of 1 may miss rapid sequential signals, but this is typically
	// acceptable for shutdown scenarios where only one signal triggers shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, m.signals...)
	defer signal.Stop(signalCh)

	m.logger.InfoContext(ctx, "shutdown manager waiting for signals",
		"signals", m.signals,
		"timeout", m.timeout,
		"registered_count", len(m.shutdowners))

	select {
	case sig := <-signalCh:
		m.logger.InfoContext(ctx, "received shutdown signal", "signal", sig)
		return m.shutdown()
	case <-ctx.Done():
		m.logger.InfoContext(ctx, "context cancelled, initiating shutdown", "error", ctx.Err())
		return m.shutdown()
	}
}

// Shutdown performs graceful shutdown of all registered components.
func (m *Manager) Shutdown() error {
	return m.shutdown()
}

// shutdown performs the actual shutdown logic with proper error handling.
func (m *Manager) shutdown() error {
	var shutdownErr error

	m.once.Do(func() {
		shutdownErr = m.performShutdown()
	})

	return shutdownErr
}

// performShutdown handles the actual shutdown logic.
func (m *Manager) performShutdown() error {
	if len(m.shutdowners) == 0 {
		m.logger.Info("no shutdowners registered, shutdown completed immediately")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	m.logger.Info("initiating graceful shutdown", "components_count", len(m.shutdowners))

	// Shutdown all components concurrently
	errCh := make(chan error, len(m.shutdowners))
	var wg sync.WaitGroup

	for i, shutdowner := range m.shutdowners {
		wg.Add(1)
		go func(s Shutdowner, idx int) {
			defer wg.Done()
			if err := s.Shutdown(ctx); err != nil {
				m.logger.ErrorContext(ctx, "component shutdown failed", "component_index", idx, "error", err)
				errCh <- err
			} else {
				m.logger.DebugContext(ctx, "component shutdown completed", "component_index", idx)
			}
		}(shutdowner, i)
	}

	// Wait for all components to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(errCh)
		// Collect shutdown errors
		var errs []error
		for err := range errCh {
			if err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			shutdownErr := errors.Join(errs...)
			m.logger.Error("components shutdown completed with errors", "error", shutdownErr)
			return shutdownErr
		}

		m.logger.Info("all components shutdown completed successfully")
		return nil

	case <-ctx.Done():
		// Timeout exceeded; components may still be shutting down in background
		m.logger.Error("shutdown timeout exceeded", "timeout", m.timeout)
		m.logger.Warn("some components may still be shutting down in background")
		return context.DeadlineExceeded
	}
}

// ListenAndShutdown is a convenience function that combines waiting for signals
// and shutting down registered components.
func ListenAndShutdown(shutdowners ...Shutdowner) error {
	manager := NewManager()
	for _, s := range shutdowners {
		manager.Register(s)
	}
	return manager.Wait()
}

// ListenAndShutdownWithTimeout is a convenience function with custom timeout.
func ListenAndShutdownWithTimeout(timeout time.Duration, shutdowners ...Shutdowner) error {
	manager := NewManager().WithTimeout(timeout)
	for _, s := range shutdowners {
		manager.Register(s)
	}
	return manager.Wait()
}
