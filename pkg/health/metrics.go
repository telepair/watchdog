package health

import (
	"compress/gzip"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// HTTPMaxRequestsInFlight limits concurrent requests to prevent resource exhaustion.
	HTTPMaxRequestsInFlight = 10
	// HTTPTimeoutSeconds is the timeout for HTTP requests in seconds.
	HTTPTimeoutSeconds = 5
)

// DefaultBuckets is the default histogram buckets used by the metrics helpers.
//
//nolint:gochecknoglobals // DefaultBuckets is a package-level configuration variable
var DefaultBuckets = prometheus.DefBuckets

// Counter defines the minimal counter interface exposed by this package.
type Counter interface {
	// Inc increments the counter by 1.
	Inc()
	// Add adds the given value to the counter. Value must be non-negative.
	Add(v float64)
}

// Gauge defines the minimal gauge interface exposed by this package.
type Gauge interface {
	// Set sets the gauge to the given value.
	Set(v float64)
	// Inc increments the gauge by 1.
	Inc()
	// Dec decrements the gauge by 1.
	Dec()
	// Add adds the given value to the gauge (can be negative).
	Add(v float64)
}

// Histogram defines the minimal histogram interface exposed by this package.
type Histogram interface {
	// Observe records one observation.
	Observe(v float64)
}

// PrometheusRegistry wraps a prometheus.Registry and provides helpers for common metric types
// with optional constant labels and a namespace.
type PrometheusRegistry struct {
	reg       *prometheus.Registry
	namespace string
	handler   atomic.Value // stores cachedHandler
	// HTTP metrics (initialized once)
	httpOnce              sync.Once
	httpInFlight          prometheus.Gauge
	httpRequestsTotal     *prometheus.CounterVec
	httpRequestDuration   *prometheus.HistogramVec
	httpRequestSizeBytes  *prometheus.HistogramVec
	httpResponseSizeBytes *prometheus.HistogramVec
}

// NewPrometheusRegistry creates a new PrometheusRegistry with the given
// namespace and registers standard Go and process collectors.
func NewPrometheusRegistry(namespace string) *PrometheusRegistry {
	r := prometheus.NewRegistry()
	// Register standard collectors
	_ = r.Register(collectors.NewGoCollector())
	_ = r.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return &PrometheusRegistry{
		reg:       r,
		namespace: namespace,
	}
}

// NewCounter creates and registers a new counter with the provided name and const labels.
func (r *PrometheusRegistry) NewCounter(name string, constLabels map[string]string) (Counter, error) {
	if r == nil || r.reg == nil {
		return nil, errors.New("registry is nil")
	}
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   r.namespace,
		Name:        name,
		ConstLabels: maps.Clone(constLabels),
	})
	if err := r.reg.Register(c); err != nil {
		return nil, fmt.Errorf("error registering metric %q: %w", name, err)
	}

	// Clear cached handler when new metrics are registered
	r.invalidateHandler()
	return &counterImpl{c: c}, nil
}

// NewGauge creates and registers a new gauge with the provided name and const labels.
func (r *PrometheusRegistry) NewGauge(name string, constLabels map[string]string) (Gauge, error) {
	if r == nil || r.reg == nil {
		return nil, errors.New("registry is nil")
	}
	g := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   r.namespace,
		Name:        name,
		ConstLabels: maps.Clone(constLabels),
	})
	if err := r.reg.Register(g); err != nil {
		return nil, fmt.Errorf("error registering metric %q: %w", name, err)
	}

	// Clear cached handler when new metrics are registered
	r.invalidateHandler()
	return &gaugeImpl{g: g}, nil
}

// NewHistogram creates and registers a new histogram with the provided name, const labels and buckets.
func (r *PrometheusRegistry) NewHistogram(
	name string,
	constLabels map[string]string,
	buckets []float64,
) (Histogram, error) {
	if r == nil || r.reg == nil {
		return nil, errors.New("registry is nil")
	}
	if len(buckets) == 0 {
		buckets = DefaultBuckets
	}
	h := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace:   r.namespace,
		Name:        name,
		Buckets:     buckets,
		ConstLabels: maps.Clone(constLabels),
	})
	if err := r.reg.Register(h); err != nil {
		return nil, fmt.Errorf("error registering metric %q: %w", name, err)
	}

	// Clear cached handler when new metrics are registered
	r.invalidateHandler()
	return &histogramImpl{h: h}, nil
}

// cachedHandler wraps the cached HTTP handler along with its readiness state.
type cachedHandler struct {
	handler http.Handler
	ready   bool
}

// HTTPHandler returns an `http.Handler` that serves metrics from this registry.
// The handler is cached after first creation for better performance and includes compression.
func (r *PrometheusRegistry) HTTPHandler() http.Handler {
	if r == nil || r.reg == nil {
		return http.NotFoundHandler()
	}

	// Fast path: atomically load cached handler
	if v := r.handler.Load(); v != nil {
		if ch, ok := v.(cachedHandler); ok && ch.ready && ch.handler != nil {
			return ch.handler
		}
	}

	// Create optimized handler with compression and caching
	baseHandler := promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{
		EnableOpenMetrics:   true,
		MaxRequestsInFlight: HTTPMaxRequestsInFlight,
		Timeout:             HTTPTimeoutSeconds * time.Second,
	})

	// Wrap with compression and security headers and store atomically
	h := r.wrapMetricsHandler(baseHandler)
	r.handler.Store(cachedHandler{handler: h, ready: true})
	return h
}

// registerHTTPGaugeMetrics registers HTTP gauge metrics.
func (r *PrometheusRegistry) registerHTTPGaugeMetrics() {
	r.httpInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: r.namespace,
		Name:      "http_in_flight_requests",
		Help:      "Current number of in-flight HTTP requests.",
	})
	_ = r.reg.Register(r.httpInFlight)
}

// registerHTTPCounterMetrics registers HTTP counter metrics.
func (r *PrometheusRegistry) registerHTTPCounterMetrics() {
	r.httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: r.namespace,
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests handled, labeled by code and method.",
	}, []string{"code", "method"})
	_ = r.reg.Register(r.httpRequestsTotal)
}

// registerHTTPHistogramMetrics registers HTTP histogram metrics.
func (r *PrometheusRegistry) registerHTTPHistogramMetrics() {
	r.httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: r.namespace,
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request latency in seconds.",
		Buckets:   DefaultBuckets,
	}, []string{"handler", "method", "code"})
	_ = r.reg.Register(r.httpRequestDuration)

	sizeBuckets := prometheus.ExponentialBuckets(200, 2, 10)

	r.httpRequestSizeBytes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: r.namespace,
		Name:      "http_request_size_bytes",
		Help:      "Size of HTTP requests in bytes.",
		Buckets:   sizeBuckets,
	}, []string{"handler"})
	_ = r.reg.Register(r.httpRequestSizeBytes)

	r.httpResponseSizeBytes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: r.namespace,
		Name:      "http_response_size_bytes",
		Help:      "Size of HTTP responses in bytes.",
		Buckets:   sizeBuckets,
	}, []string{"handler"})
	_ = r.reg.Register(r.httpResponseSizeBytes)
}

// HTTPMiddleware wraps an http.Handler with Prometheus HTTP instrumentation using global metrics.
func (r *PrometheusRegistry) HTTPMiddleware(handlerName string, h http.Handler) http.Handler {
	if r == nil || r.reg == nil || h == nil || handlerName == "" {
		return h
	}
	r.httpOnce.Do(func() {
		r.registerHTTPGaugeMetrics()
		r.registerHTTPCounterMetrics()
		r.registerHTTPHistogramMetrics()
	})

	duration := r.httpRequestDuration.MustCurryWith(prometheus.Labels{"handler": handlerName})
	reqSize := r.httpRequestSizeBytes.MustCurryWith(prometheus.Labels{"handler": handlerName})
	respSize := r.httpResponseSizeBytes.MustCurryWith(prometheus.Labels{"handler": handlerName})

	return promhttp.InstrumentHandlerInFlight(
		r.httpInFlight,
		promhttp.InstrumentHandlerDuration(
			duration,
			promhttp.InstrumentHandlerCounter(
				r.httpRequestsTotal,
				promhttp.InstrumentHandlerResponseSize(
					respSize,
					promhttp.InstrumentHandlerRequestSize(
						reqSize,
						h,
					),
				),
			),
		),
	)
}

// gzipResponseWriter wraps http.ResponseWriter to provide gzip compression.
type gzipResponseWriter struct {
	http.ResponseWriter
	gw            *gzip.Writer
	headerWritten bool
}

func (grw *gzipResponseWriter) WriteHeader(statusCode int) {
	if !grw.headerWritten {
		grw.ResponseWriter.WriteHeader(statusCode)
		grw.headerWritten = true
	}
}

func (grw *gzipResponseWriter) Write(data []byte) (int, error) {
	if !grw.headerWritten {
		grw.WriteHeader(http.StatusOK)
	}
	return grw.gw.Write(data)
}

func (grw *gzipResponseWriter) Flush() {
	if err := grw.gw.Flush(); err != nil {
		slog.Default().With("component", "health.metrics").Error("gzipResponseWriter.Flush failed", "error", err)
	}
	if flusher, ok := grw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (grw *gzipResponseWriter) Close() error {
	return grw.gw.Close()
}

// wrapMetricsHandler adds compression, security headers and caching to metrics endpoint.
func (r *PrometheusRegistry) wrapMetricsHandler(base http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")

		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if req.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}

		if strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
			r.serveWithGzipCompression(w, req, base)
		} else {
			base.ServeHTTP(w, req)
		}
	})
}

// serveWithGzipCompression serves the request with gzip compression.
func (r *PrometheusRegistry) serveWithGzipCompression(w http.ResponseWriter, req *http.Request, base http.Handler) {
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Vary", "Accept-Encoding")

	gw := gzip.NewWriter(w)
	grw := &gzipResponseWriter{
		ResponseWriter: w,
		gw:             gw,
	}

	base.ServeHTTP(grw, req)

	// Flush and close gzip writer to ensure all data is written
	if err := gw.Flush(); err != nil {
		// Log error but don't fail the request
		_ = err
	}
	if err := gw.Close(); err != nil {
		// Log error but don't fail the request
		_ = err
	}
}

// invalidateHandler clears the cached HTTP handler.
func (r *PrometheusRegistry) invalidateHandler() {
	// Reset cached handler; next access will rebuild
	r.handler.Store(cachedHandler{})
}

type counterImpl struct{ c prometheus.Counter }

// Inc increments the counter by 1.
func (c *counterImpl) Inc() { c.c.Inc() }

// Add adds the given non-negative value to the counter.
func (c *counterImpl) Add(v float64) {
	logger := slog.Default().With("component", "health.metrics")
	if v < 0 {
		// Strict error handling for negative values
		// Log as error with caller information for debugging
		logger.Error("counter.Add received negative value",
			"value", v,
			"action", "ignored",
		)
		// In production, we could panic here to fail fast
		// panic(fmt.Sprintf("counter cannot add negative value: %f", v))
		return
	}
	// Validate for NaN and Inf values
	if math.IsNaN(v) {
		logger.Error("counter.Add received NaN value", "action", "ignored")
		return
	}
	if math.IsInf(v, 0) {
		logger.Error("counter.Add received infinite value", "value", v, "action", "ignored")
		return
	}
	c.c.Add(v)
}

type gaugeImpl struct{ g prometheus.Gauge }

// Set sets the gauge to the given value.
func (g *gaugeImpl) Set(v float64) { g.g.Set(v) }

// Inc increments the gauge by 1.
func (g *gaugeImpl) Inc() { g.g.Inc() }

// Dec decrements the gauge by 1.
func (g *gaugeImpl) Dec() { g.g.Dec() }

// Add adds the given value to the gauge (can be negative).
func (g *gaugeImpl) Add(v float64) { g.g.Add(v) }

type histogramImpl struct{ h prometheus.Histogram }

// Observe records one observation to the histogram.
func (h *histogramImpl) Observe(v float64) { h.h.Observe(v) }
