package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/telepair/watchdog/pkg/utils"
)

// Default values for health configuration.
const (
	DefaultAddr      = ":9091"
	LivezPath        = "/livez"
	ReadyzPath       = "/readyz"
	MetricsPath      = "/metrics"
	MetricsNamespace = "watchdog"
)

const (
	// HTTPReadTimeout is the timeout for reading the entire request, including the body.
	HTTPReadTimeout = 5 * time.Second
	// HTTPReadHeaderTimeout is the amount of time allowed to read request headers.
	HTTPReadHeaderTimeout = 5 * time.Second
	// HTTPWriteTimeout is the timeout for writes before timing out.
	HTTPWriteTimeout = 10 * time.Second
	// HTTPIdleTimeout is the maximum amount of time to wait for the next request.
	HTTPIdleTimeout = 60 * time.Second
	// HTTPMaxHeaderBytes is the maximum number of bytes the server will read parsing request headers.
	HTTPMaxHeaderBytes = 8192 // 8KB
)

// Server hosts liveness/readiness endpoints and Prometheus metrics
// Now follows single responsibility principle by coordinating specialized components.
type Server struct {
	addr      string
	health    *Manager
	metrics   *PrometheusRegistry
	readiness *ReadinessManager
	srv       *http.Server
}

// NewServer creates a new Server with component-based architecture following SOLID principles.
func NewServer(addr string) (*Server, error) {
	if err := utils.ValidateAddr(addr); err != nil {
		return nil, fmt.Errorf("invalid address [%s]: %w", addr, err)
	}

	s := &Server{
		addr:      addr,
		health:    NewHealthManager(),
		metrics:   NewPrometheusRegistry(MetricsNamespace),
		readiness: NewReadinessManager(),
	}

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc(LivezPath, s.LivezHandler)
	mux.HandleFunc(ReadyzPath, s.ReadyzHandler)
	mux.Handle(MetricsPath, s.metrics.HTTPHandler())

	s.srv = &http.Server{
		Addr:                         addr,
		Handler:                      mux,
		ReadTimeout:                  HTTPReadTimeout,
		ReadHeaderTimeout:            HTTPReadHeaderTimeout,
		WriteTimeout:                 HTTPWriteTimeout,
		IdleTimeout:                  HTTPIdleTimeout,
		MaxHeaderBytes:               HTTPMaxHeaderBytes,
		DisableGeneralOptionsHandler: true,
	}

	// Log server initialization
	slog.Info("health server initialized",
		"addr", addr, "livez_path", LivezPath, "readyz_path", ReadyzPath, "metrics_path", MetricsPath)

	return s, nil
}

// ListenAndServe starts the server and blocks until it stops.
func (s *Server) ListenAndServe() error {
	slog.Info("starting HTTP server", "addr", s.srv.Addr)
	err := s.srv.ListenAndServe()

	switch err {
	case nil:
		slog.Info("HTTP server stopped")
	case http.ErrServerClosed:
		slog.Info("HTTP server closed")
	default:
		slog.Error("HTTP server listen failed", "error", err)
	}
	return err
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return errors.New("health server is not initialized")
	}

	slog.InfoContext(ctx, "shutting down health server")

	wg := sync.WaitGroup{}
	var httpErr, healthErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		httpErr = s.srv.Shutdown(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		healthErr = s.health.Stop(ctx)
	}()

	// Wait for all shutdown routines to complete
	wg.Wait()

	if httpErr != nil || healthErr != nil {
		slog.ErrorContext(ctx, "health server shutdown had errors",
			"http_error", httpErr, "health_error", healthErr)
		return errors.Join(httpErr, healthErr)
	}

	slog.InfoContext(ctx, "health server shutdown completed")
	return nil
}

// SetReady sets the ready state of the server.
func (s *Server) SetReady(ready bool) {
	s.readiness.SetReady(ready)
}

// ReadyzHandler handles readiness checks.
func (s *Server) ReadyzHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		slog.DebugContext(r.Context(), "readyz method not allowed", "method", r.Method)
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ready := s.readiness.IsReady()
	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	if r.Method == http.MethodHead {
		w.WriteHeader(statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	status := healthStatusOK
	if !ready {
		status = healthStatusFail
	}

	body := map[string]any{
		"status":     status,
		"ready":      ready,
		"uptime_sec": int64(s.readiness.GetUptime().Seconds()),
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.WarnContext(r.Context(), "readyz encode response failed", "error", err)
	}
	slog.DebugContext(r.Context(), "readyz responded", "status", status, "code", statusCode)
}

// LivezHandler handles liveness checks.
func (s *Server) LivezHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		slog.DebugContext(r.Context(), "livez method not allowed", "method", r.Method)
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	checks, anyFail := s.health.GetHealthStatus()
	statusCode := http.StatusOK
	overall := healthStatusOK
	if anyFail {
		statusCode = http.StatusServiceUnavailable
		overall = healthStatusFail
	}

	if r.Method == http.MethodHead {
		w.WriteHeader(statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	body := map[string]any{
		"status":     overall,
		"healthy":    !anyFail,
		"uptime_sec": int64(s.readiness.GetUptime().Seconds()),
	}
	if checks != nil {
		body["checks"] = checks
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.WarnContext(r.Context(), "livez encode response failed", "error", err)
	}
	slog.DebugContext(r.Context(), "livez responded", "overall", overall, "code", statusCode)
}

// RegisterChecker registers a health checker.
func (s *Server) RegisterChecker(name string, interval time.Duration, fn CheckFunc) error {
	return s.health.RegisterChecker(name, interval, fn)
}

// RegisterCounter registers a counter metric.
func (s *Server) RegisterCounter(name string, constLabels map[string]string) (Counter, error) {
	return s.metrics.NewCounter(name, constLabels)
}

// RegisterGauge registers a gauge metric.
func (s *Server) RegisterGauge(name string, constLabels map[string]string) (Gauge, error) {
	return s.metrics.NewGauge(name, constLabels)
}

// RegisterHistogram registers a histogram metric.
func (s *Server) RegisterHistogram(name string, constLabels map[string]string, buckets []float64) (Histogram, error) {
	return s.metrics.NewHistogram(name, constLabels, buckets)
}
