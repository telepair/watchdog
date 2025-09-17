package health

import (
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestNewPrometheusRegistry(t *testing.T) {
	t.Parallel()

	namespace := "test_namespace"
	registry := NewPrometheusRegistry(namespace)

	if registry == nil {
		t.Fatal("NewPrometheusRegistry returned nil")
	}

	if registry.reg == nil {
		t.Error("prometheus registry not initialized")
	}

	if registry.namespace != namespace {
		t.Errorf("expected namespace %q, got %q", namespace, registry.namespace)
	}
}

func TestPrometheusRegistry_NewCounter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		registry    *PrometheusRegistry
		metricName  string
		constLabels map[string]string
		wantErr     bool
	}{
		{
			name:        "valid counter",
			registry:    NewPrometheusRegistry("test"),
			metricName:  "test_counter",
			constLabels: map[string]string{"label": "value"},
			wantErr:     false,
		},
		{
			name:        "duplicate counter",
			registry:    NewPrometheusRegistry("test"),
			metricName:  "duplicate_counter",
			constLabels: map[string]string{},
			wantErr:     false, // First registration should succeed
		},
		{
			name:        "nil registry",
			registry:    nil,
			metricName:  "test_counter",
			constLabels: map[string]string{},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.name == "duplicate counter" {
				// Register first counter
				_, err := tt.registry.NewCounter(tt.metricName, tt.constLabels)
				if err != nil {
					t.Fatalf("first registration failed: %v", err)
				}
				// Try to register again - should fail
				_, err = tt.registry.NewCounter(tt.metricName, tt.constLabels)
				if err == nil {
					t.Error("expected error for duplicate registration")
				}
				return
			}

			counter, err := tt.registry.NewCounter(tt.metricName, tt.constLabels)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCounter() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && counter == nil {
				t.Error("expected non-nil counter")
			}
		})
	}
}

func TestPrometheusRegistry_NewGauge(t *testing.T) {
	t.Parallel()

	registry := NewPrometheusRegistry("test")
	constLabels := map[string]string{"env": "test"}

	gauge, err := registry.NewGauge("test_gauge", constLabels)
	if err != nil {
		t.Fatalf("NewGauge() error = %v", err)
	}

	if gauge == nil {
		t.Error("expected non-nil gauge")
	}

	// Test gauge operations
	gauge.Set(42.0)
	gauge.Inc()
	gauge.Dec()
	gauge.Add(10.5)
}

func TestPrometheusRegistry_NewHistogram(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		metricName  string
		constLabels map[string]string
		buckets     []float64
		wantErr     bool
	}{
		{
			name:        "valid histogram with buckets",
			metricName:  "test_histogram",
			constLabels: map[string]string{"type": "test"},
			buckets:     []float64{0.1, 0.5, 1.0, 5.0},
			wantErr:     false,
		},
		{
			name:        "histogram with default buckets",
			metricName:  "test_histogram_default",
			constLabels: map[string]string{},
			buckets:     nil,
			wantErr:     false,
		},
		{
			name:        "histogram with empty buckets",
			metricName:  "test_histogram_empty",
			constLabels: map[string]string{},
			buckets:     []float64{},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			registry := NewPrometheusRegistry("test")
			histogram, err := registry.NewHistogram(tt.metricName, tt.constLabels, tt.buckets)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewHistogram() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if histogram == nil {
					t.Error("expected non-nil histogram")
				}
				// Test histogram operations
				histogram.Observe(0.5)
				histogram.Observe(1.5)
			}
		})
	}
}

func TestPrometheusRegistry_HTTPHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		registry *PrometheusRegistry
		method   string
		headers  map[string]string
		wantCode int
	}{
		{
			name:     "GET request",
			registry: NewPrometheusRegistry("test"),
			method:   http.MethodGet,
			headers:  map[string]string{},
			wantCode: http.StatusOK,
		},
		{
			name:     "HEAD request",
			registry: NewPrometheusRegistry("test"),
			method:   http.MethodHead,
			headers:  map[string]string{},
			wantCode: http.StatusOK,
		},
		{
			name:     "POST request",
			registry: NewPrometheusRegistry("test"),
			method:   http.MethodPost,
			headers:  map[string]string{},
			wantCode: http.StatusMethodNotAllowed,
		},
		{
			name:     "GET with gzip compression",
			registry: NewPrometheusRegistry("test"),
			method:   http.MethodGet,
			headers:  map[string]string{"Accept-Encoding": "gzip, deflate"},
			wantCode: http.StatusOK,
		},
		{
			name:     "nil registry",
			registry: nil,
			method:   http.MethodGet,
			headers:  map[string]string{},
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var handler http.Handler
			if tt.registry != nil {
				// Add a counter to make the output non-empty
				_, _ = tt.registry.NewCounter("test_metric", nil)
				handler = tt.registry.HTTPHandler()
			} else {
				handler = (&PrometheusRegistry{}).HTTPHandler()
			}

			req := httptest.NewRequest(tt.method, "/metrics", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantCode {
				t.Errorf("expected status code %d, got %d", tt.wantCode, recorder.Code)
			}

			// Check security headers
			if tt.wantCode == http.StatusOK {
				expectedHeaders := map[string]string{
					"X-Content-Type-Options": "nosniff",
					"X-Frame-Options":        "DENY",
					"Cache-Control":          "no-cache, no-store, must-revalidate",
					"Pragma":                 "no-cache",
				}

				for header, expectedValue := range expectedHeaders {
					if got := recorder.Header().Get(header); got != expectedValue {
						t.Errorf("header %s: expected %q, got %q", header, expectedValue, got)
					}
				}

				// Check compression
				if strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") && tt.method == http.MethodGet {
					if recorder.Header().Get("Content-Encoding") != "gzip" {
						t.Error("expected gzip content encoding")
					}
					if recorder.Header().Get("Vary") != "Accept-Encoding" {
						t.Error("expected Vary: Accept-Encoding header")
					}
				}
			}
		})
	}
}

func TestPrometheusRegistry_InstrumentHTTPHandler(t *testing.T) {
	t.Parallel()

	registry := NewPrometheusRegistry("test")

	// Create a simple handler
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	instrumentedHandler := registry.InstrumentHTTPHandler("test_handler", baseHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()

	instrumentedHandler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}

	if body := recorder.Body.String(); body != "OK" {
		t.Errorf("expected body 'OK', got %q", body)
	}
}

func TestCounterImpl(t *testing.T) {
	t.Parallel()

	registry := NewPrometheusRegistry("test")
	counter, err := registry.NewCounter("test_counter", nil)
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}

	// Test Inc
	counter.Inc()

	// Test Add with valid values
	counter.Add(5.5)
	counter.Add(0)

	// Test Add with invalid values
	counter.Add(-1.0)        // Should be ignored
	counter.Add(math.NaN())  // Should be ignored
	counter.Add(math.Inf(1)) // Should be ignored
}

func TestGaugeImpl(t *testing.T) {
	t.Parallel()

	registry := NewPrometheusRegistry("test")
	gauge, err := registry.NewGauge("test_gauge", nil)
	if err != nil {
		t.Fatalf("failed to create gauge: %v", err)
	}

	// Test all gauge operations
	gauge.Set(42.5)
	gauge.Inc()
	gauge.Dec()
	gauge.Add(10.0)
	gauge.Add(-5.0) // Gauges allow negative values
}

func TestHistogramImpl(t *testing.T) {
	t.Parallel()

	registry := NewPrometheusRegistry("test")
	histogram, err := registry.NewHistogram("test_histogram", nil, nil)
	if err != nil {
		t.Fatalf("failed to create histogram: %v", err)
	}

	// Test Observe
	histogram.Observe(0.1)
	histogram.Observe(0.5)
	histogram.Observe(1.0)
	histogram.Observe(2.5)
}

func TestNewMetricsManager(t *testing.T) {
	t.Parallel()

	namespace := "test_metrics"
	manager := NewMetricsManager(namespace)

	if manager == nil {
		t.Fatal("NewMetricsManager returned nil")
	}

	if manager.reg == nil {
		t.Error("registry not initialized")
	}

	if manager.logger == nil {
		t.Error("logger not initialized")
	}
}

func TestMetricsManager_RegisterCounter(t *testing.T) {
	t.Parallel()

	manager := NewMetricsManager("test")
	labels := map[string]string{"service": "api"}

	counter, err := manager.RegisterCounter("requests_total", labels)
	if err != nil {
		t.Fatalf("failed to register counter: %v", err)
	}

	if counter == nil {
		t.Error("expected non-nil counter")
	}

	// Test counter operations
	counter.Inc()
	counter.Add(5.0)
}

func TestMetricsManager_RegisterGauge(t *testing.T) {
	t.Parallel()

	manager := NewMetricsManager("test")
	labels := map[string]string{"type": "memory"}

	gauge, err := manager.RegisterGauge("usage_bytes", labels)
	if err != nil {
		t.Fatalf("failed to register gauge: %v", err)
	}

	if gauge == nil {
		t.Error("expected non-nil gauge")
	}

	// Test gauge operations
	gauge.Set(1024000)
	gauge.Inc()
}

func TestMetricsManager_RegisterHistogram(t *testing.T) {
	t.Parallel()

	manager := NewMetricsManager("test")
	labels := map[string]string{"endpoint": "/api/v1"}
	buckets := []float64{0.1, 0.5, 1.0, 5.0}

	histogram, err := manager.RegisterHistogram("request_duration_seconds", labels, buckets)
	if err != nil {
		t.Fatalf("failed to register histogram: %v", err)
	}

	if histogram == nil {
		t.Error("expected non-nil histogram")
	}

	// Test histogram operations
	histogram.Observe(0.25)
	histogram.Observe(1.5)
}

func TestMetricsManager_HTTPHandler(t *testing.T) {
	t.Parallel()

	manager := NewMetricsManager("test")

	// Register some metrics
	_, _ = manager.RegisterCounter("test_counter", nil)
	_, _ = manager.RegisterGauge("test_gauge", nil)

	handler := manager.HTTPHandler()
	if handler == nil {
		t.Error("expected non-nil handler")
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}
}

func TestMetricsManager_HTTPMiddleware(t *testing.T) {
	t.Parallel()

	manager := NewMetricsManager("test")

	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	middleware := manager.HTTPMiddleware("test_api", baseHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	recorder := httptest.NewRecorder()

	middleware.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}

	if body := recorder.Body.String(); body != "test response" {
		t.Errorf("expected 'test response', got %q", body)
	}
}

func TestGzipResponseWriter(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	gzipWriter := gzip.NewWriter(recorder)
	grw := &gzipResponseWriter{
		ResponseWriter: recorder,
		gw:             gzipWriter,
	}

	// Test Write
	data := []byte("test data")
	n, err := grw.Write(data)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("expected to write %d bytes, wrote %d", len(data), n)
	}

	// Test Flush
	grw.Flush()

	// Test Close
	err = grw.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestPrometheusRegistry_UnregisterCollector(t *testing.T) {
	t.Parallel()

	registry := NewPrometheusRegistry("test")

	// Create and register a counter
	_, collector, err := registry.NewCounterWithCollector("test_counter", nil)
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}

	// Unregister the collector
	success := registry.UnregisterCollector(collector)
	if !success {
		t.Error("expected successful unregistration")
	}

	// Try to unregister again - should fail
	success = registry.UnregisterCollector(collector)
	if success {
		t.Error("expected failed unregistration for already unregistered collector")
	}

	// Test with nil registry
	var nilRegistry *PrometheusRegistry
	success = nilRegistry.UnregisterCollector(collector)
	if success {
		t.Error("expected failed unregistration for nil registry")
	}
}

func TestPrometheusRegistry_CachedHandler(t *testing.T) {
	t.Parallel()

	registry := NewPrometheusRegistry("test")

	// First call should create and cache the handler
	handler1 := registry.HTTPHandler()
	if handler1 == nil {
		t.Error("expected non-nil handler")
	}

	// Second call should return the same cached handler
	handler2 := registry.HTTPHandler()
	// Note: Cannot directly compare http.HandlerFunc instances, but we can verify
	// they both exist and are not nil
	if handler2 == nil {
		t.Error("expected non-nil cached handler")
	}

	// Registering a new metric should invalidate the cache
	_, err := registry.NewCounter("invalidate_test", nil)
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}

	// Next call should create a new handler
	handler3 := registry.HTTPHandler()
	if handler3 == nil {
		t.Error("expected non-nil handler after cache invalidation")
	}
}

func TestPrometheusRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	registry := NewPrometheusRegistry("test")
	const numGoroutines = 10
	const numMetrics = 5

	var wg sync.WaitGroup

	// Concurrently register metrics
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range numMetrics {
				metricName := fmt.Sprintf("metric_%d_%d", id, j)
				_, err := registry.NewCounter(metricName, nil)
				if err != nil {
					t.Errorf("failed to register metric %s: %v", metricName, err)
				}
			}
		}(i)
	}

	// Concurrently access HTTP handler
	for i := range numGoroutines {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			for range 10 {
				handler := registry.HTTPHandler()
				if handler == nil {
					t.Error("expected non-nil handler")
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestGzipCompression(t *testing.T) {
	t.Parallel()

	registry := NewPrometheusRegistry("test")
	_, _ = registry.NewCounter("test_metric", nil)

	handler := registry.HTTPHandler()

	// Test with gzip acceptance
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}

	if recorder.Header().Get("Content-Encoding") != "gzip" {
		t.Error("expected gzip content encoding")
	}

	// Verify the response is actually gzipped
	body := recorder.Body.Bytes()
	if len(body) == 0 {
		t.Error("expected non-empty response body")
	}

	// Try to decompress
	gzipReader, err := gzip.NewReader(recorder.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzipReader.Close()

	decompressed, err := io.ReadAll(gzipReader)
	if err != nil {
		t.Fatalf("failed to decompress response: %v", err)
	}

	if len(decompressed) == 0 {
		t.Error("expected non-empty decompressed content")
	}
}
