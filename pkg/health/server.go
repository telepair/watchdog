package health

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"
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

// HTTPServer handles HTTP server operations following single responsibility principle.
type HTTPServer struct {
	server *http.Server
	logger *slog.Logger
}

// NewHTTPServer creates a new HTTP server.
func NewHTTPServer(addr string, handler http.Handler) *HTTPServer {
	return &HTTPServer{
		server: &http.Server{
			Addr:                         addr,
			Handler:                      handler,
			ReadTimeout:                  HTTPReadTimeout,
			ReadHeaderTimeout:            HTTPReadHeaderTimeout,
			WriteTimeout:                 HTTPWriteTimeout,
			IdleTimeout:                  HTTPIdleTimeout,
			MaxHeaderBytes:               HTTPMaxHeaderBytes,
			DisableGeneralOptionsHandler: true,
		},
		logger: slog.Default().With("component", "health.http_server"),
	}
}

// ListenAndServe starts the HTTP server.
func (h *HTTPServer) ListenAndServe(ctx context.Context) error {
	if h == nil || h.server == nil {
		return errors.New("HTTP server is not initialized")
	}

	h.logger.InfoContext(ctx, "starting HTTP server", "addr", h.server.Addr)
	err := h.server.ListenAndServe()

	switch err {
	case nil:
		h.logger.InfoContext(ctx, "HTTP server stopped")
	case http.ErrServerClosed:
		h.logger.InfoContext(ctx, "HTTP server closed")
	default:
		h.logger.ErrorContext(ctx, "HTTP server listen failed", "error", err)
	}

	return err
}

// Shutdown gracefully shuts down the HTTP server.
func (h *HTTPServer) Shutdown(ctx context.Context) error {
	if h == nil || h.server == nil {
		if h != nil && h.logger != nil {
			h.logger.ErrorContext(ctx, "HTTP server is not initialized")
		}
		return errors.New("HTTP server is not initialized")
	}

	h.logger.InfoContext(ctx, "shutting down HTTP server")
	if err := h.server.Shutdown(ctx); err != nil {
		h.logger.ErrorContext(ctx, "HTTP server shutdown failed", "error", err)
		return err
	}
	h.logger.InfoContext(ctx, "HTTP server shutdown completed")
	return nil
}

// Coordinator coordinates health-related components following single responsibility principle.
type Coordinator struct {
	health    *Manager
	metrics   *MetricsManager
	readiness *ReadinessManager
	logger    *slog.Logger
}

// NewCoordinator creates a new health coordinator.
func NewCoordinator(metricsNamespace string) *Coordinator {
	return &Coordinator{
		health:    NewHealthManager(),
		metrics:   NewMetricsManager(metricsNamespace),
		readiness: NewReadinessManager(),
		logger:    slog.Default().With("component", "health.coordinator"),
	}
}

// SetReady sets the ready state.
func (c *Coordinator) SetReady(ready bool) {
	c.logger.InfoContext(context.Background(), "setting readiness state", "ready", ready)
	c.readiness.SetReady(ready)
}

// RegisterChecker registers a health checker.
func (c *Coordinator) RegisterChecker(name string, interval time.Duration, fn CheckFunc) error {
	c.logger.InfoContext(context.Background(), "registering health checker", "name", name, "interval", interval)
	return c.health.RegisterChecker(name, interval, fn)
}

// RegisterCounter registers a counter metric.
func (c *Coordinator) RegisterCounter(name string, constLabels map[string]string) (Counter, error) {
	c.logger.InfoContext(context.Background(), "registering counter metric", "name", name, "constLabels", constLabels)
	return c.metrics.RegisterCounter(name, constLabels)
}

// RegisterGauge registers a gauge metric.
func (c *Coordinator) RegisterGauge(name string, constLabels map[string]string) (Gauge, error) {
	c.logger.InfoContext(context.Background(), "registering gauge metric", "name", name, "constLabels", constLabels)
	return c.metrics.RegisterGauge(name, constLabels)
}

// RegisterHistogram registers a histogram metric.
func (c *Coordinator) RegisterHistogram(name string, constLabels map[string]string,
	buckets []float64) (Histogram, error) {
	c.logger.InfoContext(context.Background(), "registering histogram metric",
		"name", name, "constLabels", constLabels, "buckets", buckets)
	return c.metrics.RegisterHistogram(name, constLabels, buckets)
}

// Shutdown gracefully shuts down all components.
func (c *Coordinator) Shutdown(ctx context.Context) error {
	c.readiness.SetReady(false)
	c.logger.InfoContext(ctx, "set readiness to false for graceful shutdown")

	if err := c.health.Stop(ctx); err != nil {
		c.logger.ErrorContext(ctx, "health manager shutdown failed", "error", err)
		return err
	}
	c.logger.InfoContext(ctx, "health coordinator shutdown completed")
	return nil
}

// Handlers provides HTTP handlers for health endpoints.
type Handlers struct {
	coordinator *Coordinator
	logger      *slog.Logger
}

// NewHandlers creates new health handlers.
func NewHandlers(coordinator *Coordinator) *Handlers {
	return &Handlers{
		coordinator: coordinator,
		logger:      slog.Default().With("component", "health.handlers"),
	}
}

// ReadyzHandler handles readiness checks.
func (h *Handlers) ReadyzHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.logger.DebugContext(r.Context(), "readyz method not allowed", "method", r.Method)
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ready := h.coordinator.readiness.IsReady()
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
		"uptime_sec": int64(h.coordinator.readiness.GetUptime().Seconds()),
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		h.logger.WarnContext(r.Context(), "readyz encode response failed", "error", err)
	}
	h.logger.DebugContext(r.Context(), "readyz responded", "status", status, "code", statusCode)
}

// LivezHandler handles liveness checks.
func (h *Handlers) LivezHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		h.logger.DebugContext(r.Context(), "livez method not allowed", "method", r.Method)
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	checks, anyFail := h.coordinator.health.GetHealthStatus()
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
		"uptime_sec": int64(h.coordinator.readiness.GetUptime().Seconds()),
	}
	if checks != nil {
		body["checks"] = checks
	}

	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		h.logger.WarnContext(r.Context(), "livez encode response failed", "error", err)
	}
	h.logger.DebugContext(r.Context(), "livez responded", "overall", overall, "code", statusCode)
}

// Server hosts liveness/readiness endpoints and Prometheus metrics
// Now follows single responsibility principle by coordinating specialized components.
type Server struct {
	cfg         Config
	coordinator *Coordinator
	handlers    *Handlers
	httpServer  *HTTPServer
	logger      *slog.Logger
}

// NewServer creates a new Server with component-based architecture following SOLID principles.
func NewServer(cfg Config) (*Server, error) {
	if err := cfg.Parse(); err != nil {
		return nil, err
	}

	// Create specialized components
	coordinator := NewCoordinator(cfg.MetricsNamespace)
	handlers := NewHandlers(coordinator)

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc(cfg.LivezPath, handlers.LivezHandler)
	mux.HandleFunc(cfg.ReadyzPath, handlers.ReadyzHandler)
	mux.Handle(cfg.MetricsPath, coordinator.metrics.HTTPHandler())

	// Create HTTP server
	httpServer := NewHTTPServer(cfg.Addr, mux)

	s := &Server{
		cfg:         cfg,
		coordinator: coordinator,
		handlers:    handlers,
		httpServer:  httpServer,
		logger:      slog.Default().With("component", "health.server"),
	}

	// Log server initialization
	s.logger.InfoContext(context.Background(), "health server initialized",
		"addr", cfg.Addr,
		"livez_path", cfg.LivezPath,
		"readyz_path", cfg.ReadyzPath,
		"metrics_path", cfg.MetricsPath,
		"metrics_namespace", cfg.MetricsNamespace,
	)

	return s, nil
}

// ListenAndServe starts the server and blocks until it stops.
func (s *Server) ListenAndServe(ctx context.Context) error {
	return s.httpServer.ListenAndServe(ctx)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return errors.New("health server is not initialized")
	}

	s.logger.InfoContext(ctx, "shutting down health server")

	wg := sync.WaitGroup{}
	var httpErr, coordinatorErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		httpErr = s.httpServer.Shutdown(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		coordinatorErr = s.coordinator.Shutdown(ctx)
	}()

	// Wait for all shutdown routines to complete
	wg.Wait()

	if httpErr != nil || coordinatorErr != nil {
		s.logger.ErrorContext(ctx, "health server shutdown had errors",
			"http_error", httpErr, "coordinator_error", coordinatorErr)
		return errors.Join(httpErr, coordinatorErr)
	}

	s.logger.InfoContext(ctx, "health server shutdown completed")
	return nil
}

// SetReady sets the ready state of the server.
func (s *Server) SetReady(ready bool) {
	s.coordinator.SetReady(ready)
}

// RegisterChecker registers a health checker.
func (s *Server) RegisterChecker(name string, interval time.Duration, fn CheckFunc) error {
	return s.coordinator.RegisterChecker(name, interval, fn)
}

// RegisterCounter registers a counter metric.
func (s *Server) RegisterCounter(name string, constLabels map[string]string) (Counter, error) {
	return s.coordinator.RegisterCounter(name, constLabels)
}

// RegisterGauge registers a gauge metric.
func (s *Server) RegisterGauge(name string, constLabels map[string]string) (Gauge, error) {
	return s.coordinator.RegisterGauge(name, constLabels)
}

// RegisterHistogram registers a histogram metric.
func (s *Server) RegisterHistogram(name string, constLabels map[string]string, buckets []float64) (Histogram, error) {
	return s.coordinator.RegisterHistogram(name, constLabels, buckets)
}
