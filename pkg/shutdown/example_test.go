package shutdown_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/telepair/watchdog/pkg/shutdown"
)

// ExampleManager demonstrates how to use the shutdown manager with multiple components.
func ExampleManager() {
	logger := slog.Default()

	// Create components that need graceful shutdown
	httpServer := NewHTTPServer(":8080", logger)
	dbConnection := NewDatabaseConnection("primary", logger)

	// Create shutdown manager with custom configuration
	manager := shutdown.NewManager().
		WithTimeout(10 * time.Second).
		WithLogger(logger)

	// Register components
	manager.Register(httpServer)
	manager.Register(dbConnection)

	// Register a cleanup function
	manager.RegisterFunc(func(_ context.Context) error {
		logger.Info("performing final cleanup")
		// Simulate cleanup work
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	// For example purposes, trigger shutdown immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Trigger immediate shutdown

	// Wait for shutdown via context cancellation instead of signals
	if err := manager.WaitWithContext(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	fmt.Println("Application shutdown completed successfully")
	// Output: Application shutdown completed successfully
}

// ExampleFunc demonstrates how to use shutdown.Func for simple cleanup functions.
func ExampleFunc() {
	logger := slog.Default()

	// Create a shutdown function
	cleanupFunc := shutdown.Func(func(ctx context.Context) error {
		logger.Info("cleaning up temporary files")

		// Simulate cleanup work with context awareness
		select {
		case <-time.After(200 * time.Millisecond):
			logger.Info("cleanup completed")
			return nil
		case <-ctx.Done():
			logger.Warn("cleanup interrupted", "error", ctx.Err())
			return ctx.Err()
		}
	})

	// Use the function directly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := cleanupFunc.Shutdown(ctx); err != nil {
		fmt.Println("Cleanup failed:", err)
	} else {
		fmt.Println("Cleanup completed successfully")
	}
	// Output: Cleanup completed successfully
}

// ExampleListenAndShutdown demonstrates the convenience function for simple use cases.
func ExampleListenAndShutdown() {
	logger := slog.Default()

	// Create some components
	db1 := NewDatabaseConnection("cache", logger)
	db2 := NewDatabaseConnection("primary", logger)

	// For example purposes, demonstrate setup only
	manager := shutdown.NewManager()
	manager.Register(db1)
	manager.Register(db2)

	// In real applications, this would wait for signals:
	// if err := shutdown.ListenAndShutdown(db1, db2); err != nil {
	//     logger.Error("shutdown failed", "error", err)
	// }

	fmt.Println("Shutdown manager configured with 2 components")
	// Output: Shutdown manager configured with 2 components
}

// ExampleListenAndShutdownWithTimeout demonstrates the convenience function with custom timeout.
func ExampleListenAndShutdownWithTimeout() {
	logger := slog.Default()

	// Create components
	httpServer := NewHTTPServer(":9090", logger)
	dbConnection := NewDatabaseConnection("analytics", logger)

	// For example purposes, demonstrate the setup
	manager := shutdown.NewManager().WithTimeout(5 * time.Second)
	manager.Register(httpServer)
	manager.Register(dbConnection)

	// In real applications, this would start server and wait for signals:
	// go func() {
	//     if err := httpServer.Start(); err != http.ErrServerClosed {
	//         logger.Error("server error", "error", err)
	//     }
	// }()
	// if err := shutdown.ListenAndShutdownWithTimeout(5*time.Second, httpServer, dbConnection); err != nil {
	//     logger.Error("shutdown failed", "error", err)
	// }

	fmt.Println("Shutdown manager configured with 5s timeout")
	// Output: Shutdown manager configured with 5s timeout
}

// ExampleManager_withContextCancellation demonstrates graceful shutdown triggered by context cancellation.
func ExampleManager_withContextCancellation() {
	logger := slog.Default()

	// Create shutdown manager
	manager := shutdown.NewManager().
		WithTimeout(3 * time.Second).
		WithLogger(logger)

	// Register a long-running component
	manager.RegisterFunc(func(ctx context.Context) error {
		logger.InfoContext(ctx, "starting graceful cleanup")
		// Simulate work that respects context cancellation
		select {
		case <-time.After(1 * time.Second):
			logger.InfoContext(ctx, "cleanup completed")
			return nil
		case <-ctx.Done():
			logger.InfoContext(ctx, "cleanup cancelled", "error", ctx.Err())
			return ctx.Err()
		}
	})

	// Create a context that will be cancelled to trigger shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Simulate some application work, then trigger shutdown
	go func() {
		time.Sleep(500 * time.Millisecond)
		logger.Info("triggering shutdown via context cancellation")
		cancel()
	}()

	// Wait with context - shutdown will be triggered when context is cancelled
	if err := manager.WaitWithContext(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	fmt.Println("Context-triggered shutdown completed")
	// Output: Context-triggered shutdown completed
}

// HTTPServer is an example HTTP server that implements graceful shutdown.
type HTTPServer struct {
	server *http.Server
	logger *slog.Logger
}

// NewHTTPServer creates a new HTTP server.
func NewHTTPServer(addr string, logger *slog.Logger) *HTTPServer {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	return &HTTPServer{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		logger: logger,
	}
}

// Start starts the HTTP server.
func (s *HTTPServer) Start() error {
	s.logger.Info("starting HTTP server", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown implements the shutdown.Shutdowner interface.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.logger.InfoContext(ctx, "shutting down HTTP server")
	return s.server.Shutdown(ctx)
}

// DatabaseConnection simulates a database connection that needs graceful shutdown.
type DatabaseConnection struct {
	name   string
	logger *slog.Logger
	closed bool
	mu     sync.RWMutex
}

// NewDatabaseConnection creates a new database connection.
func NewDatabaseConnection(name string, logger *slog.Logger) *DatabaseConnection {
	return &DatabaseConnection{
		name:   name,
		logger: logger,
	}
}

// Shutdown implements the shutdown.Shutdowner interface.
func (db *DatabaseConnection) Shutdown(ctx context.Context) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil
	}

	db.logger.InfoContext(ctx, "closing database connection", "name", db.name)

	// Simulate cleanup time
	select {
	case <-time.After(100 * time.Millisecond):
		db.closed = true
		db.logger.InfoContext(ctx, "database connection closed", "name", db.name)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
