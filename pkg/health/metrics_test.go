package health

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewPrometheusRegistry(t *testing.T) {
	r := NewPrometheusRegistry("test")
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if r.namespace != "test" {
		t.Errorf("expected namespace 'test', got %q", r.namespace)
	}
	if r.reg == nil {
		t.Fatal("expected non-nil prometheus registry")
	}
}

func TestPrometheusRegistry_NewCounter(t *testing.T) {
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
			name:        "nil registry",
			registry:    nil,
			metricName:  "test_counter",
			constLabels: nil,
			wantErr:     true,
		},
		{
			name:        "nil prometheus registry",
			registry:    &PrometheusRegistry{},
			metricName:  "test_counter",
			constLabels: nil,
			wantErr:     true,
		},
		{
			name:        "duplicate counter",
			registry:    NewPrometheusRegistry("test"),
			metricName:  "duplicate_counter",
			constLabels: nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "duplicate counter" {
				// First registration should succeed
				_, err := tt.registry.NewCounter(tt.metricName, tt.constLabels)
				if err != nil {
					t.Fatalf("first registration should succeed: %v", err)
				}
			}

			counter, err := tt.registry.NewCounter(tt.metricName, tt.constLabels)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if counter == nil {
				t.Error("expected non-nil counter")
			}
		})
	}
}

func TestPrometheusRegistry_NewGauge(t *testing.T) {
	tests := []struct {
		name        string
		registry    *PrometheusRegistry
		metricName  string
		constLabels map[string]string
		wantErr     bool
	}{
		{
			name:        "valid gauge",
			registry:    NewPrometheusRegistry("test"),
			metricName:  "test_gauge",
			constLabels: map[string]string{"label": "value"},
			wantErr:     false,
		},
		{
			name:        "nil registry",
			registry:    nil,
			metricName:  "test_gauge",
			constLabels: nil,
			wantErr:     true,
		},
		{
			name:        "nil prometheus registry",
			registry:    &PrometheusRegistry{},
			metricName:  "test_gauge",
			constLabels: nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gauge, err := tt.registry.NewGauge(tt.metricName, tt.constLabels)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if gauge == nil {
				t.Error("expected non-nil gauge")
			}
		})
	}
}

func TestPrometheusRegistry_NewHistogram(t *testing.T) {
	tests := []struct {
		name        string
		registry    *PrometheusRegistry
		metricName  string
		constLabels map[string]string
		buckets     []float64
		wantErr     bool
	}{
		{
			name:        "valid histogram with custom buckets",
			registry:    NewPrometheusRegistry("test"),
			metricName:  "test_histogram",
			constLabels: map[string]string{"label": "value"},
			buckets:     []float64{0.1, 0.5, 1.0, 5.0},
			wantErr:     false,
		},
		{
			name:        "valid histogram with default buckets",
			registry:    NewPrometheusRegistry("test"),
			metricName:  "test_histogram_default",
			constLabels: nil,
			buckets:     nil,
			wantErr:     false,
		},
		{
			name:        "nil registry",
			registry:    nil,
			metricName:  "test_histogram",
			constLabels: nil,
			buckets:     nil,
			wantErr:     true,
		},
		{
			name:        "nil prometheus registry",
			registry:    &PrometheusRegistry{},
			metricName:  "test_histogram",
			constLabels: nil,
			buckets:     nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			histogram, err := tt.registry.NewHistogram(tt.metricName, tt.constLabels, tt.buckets)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if histogram == nil {
				t.Error("expected non-nil histogram")
			}
		})
	}
}

func TestCounterImpl(t *testing.T) {
	r := NewPrometheusRegistry("test")
	counter, err := r.NewCounter("test_counter", nil)
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}

	// Test Inc
	counter.Inc()
	counter.Inc()

	// Test Add with valid values
	counter.Add(5.0)
	counter.Add(0.0) // zero should be accepted

	// Test Add with invalid values (should be ignored)
	counter.Add(-1.0)         // negative
	counter.Add(math.NaN())   // NaN
	counter.Add(math.Inf(1))  // positive infinity
	counter.Add(math.Inf(-1)) // negative infinity

	// Verify the counter value is 7 (2 Inc + 5.0 Add + 0.0 Add)
	expected := `
		# HELP test_test_counter
		# TYPE test_test_counter counter
		test_test_counter 7
	`
	if err := testutil.GatherAndCompare(r.reg, strings.NewReader(expected), "test_test_counter"); err != nil {
		t.Errorf("counter value mismatch: %v", err)
	}
}

func TestGaugeImpl(t *testing.T) {
	r := NewPrometheusRegistry("test")
	gauge, err := r.NewGauge("test_gauge", nil)
	if err != nil {
		t.Fatalf("failed to create gauge: %v", err)
	}

	// Test Set
	gauge.Set(10.0)

	// Test Inc and Dec
	gauge.Inc()
	gauge.Dec()

	// Test Add (can be negative)
	gauge.Add(5.0)
	gauge.Add(-3.0)

	// Verify the gauge value is 12 (10 + 1 - 1 + 5 - 3)
	expected := `
		# HELP test_test_gauge
		# TYPE test_test_gauge gauge
		test_test_gauge 12
	`
	if err := testutil.GatherAndCompare(r.reg, strings.NewReader(expected), "test_test_gauge"); err != nil {
		t.Errorf("gauge value mismatch: %v", err)
	}
}

func TestHistogramImpl(t *testing.T) {
	r := NewPrometheusRegistry("test")
	histogram, err := r.NewHistogram("test_histogram", nil, []float64{0.1, 0.5, 1.0})
	if err != nil {
		t.Fatalf("failed to create histogram: %v", err)
	}

	// Test Observe
	histogram.Observe(0.05)
	histogram.Observe(0.3)
	histogram.Observe(0.8)
	histogram.Observe(1.5)

	// Check that metrics are registered (basic validation)
	metrics, err := r.reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	found := false
	for _, mf := range metrics {
		if mf.GetName() == "test_test_histogram" {
			found = true
			break
		}
	}
	if !found {
		t.Error("histogram metric not found in registry")
	}
}

func TestPrometheusRegistry_HTTPHandler(t *testing.T) {
	tests := []struct {
		name     string
		registry *PrometheusRegistry
		wantCode int
	}{
		{
			name:     "valid registry",
			registry: NewPrometheusRegistry("test"),
			wantCode: http.StatusOK,
		},
		{
			name:     "nil registry",
			registry: nil,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "nil prometheus registry",
			registry: &PrometheusRegistry{},
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := tt.registry.HTTPHandler()

			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("expected status code %d, got %d", tt.wantCode, w.Code)
			}
		})
	}
}

func TestPrometheusRegistry_HTTPHandler_Methods(t *testing.T) {
	r := NewPrometheusRegistry("test")
	handler := r.HTTPHandler()

	tests := []struct {
		method   string
		wantCode int
	}{
		{http.MethodGet, http.StatusOK},
		{http.MethodHead, http.StatusOK},
		{http.MethodPost, http.StatusMethodNotAllowed},
		{http.MethodPut, http.StatusMethodNotAllowed},
		{http.MethodDelete, http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/metrics", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("expected status code %d, got %d", tt.wantCode, w.Code)
			}
		})
	}
}

func TestPrometheusRegistry_HTTPHandler_Compression(t *testing.T) {
	r := NewPrometheusRegistry("test")
	handler := r.HTTPHandler()

	// Add a counter to have some metrics
	counter, err := r.NewCounter("test_counter", nil)
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}
	counter.Inc()

	tests := []struct {
		name           string
		acceptEncoding string
		expectGzip     bool
	}{
		{
			name:           "with gzip support",
			acceptEncoding: "gzip, deflate",
			expectGzip:     true,
		},
		{
			name:           "without gzip support",
			acceptEncoding: "deflate",
			expectGzip:     false,
		},
		{
			name:           "no accept encoding",
			acceptEncoding: "",
			expectGzip:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, w.Code)
			}

			if tt.expectGzip {
				if w.Header().Get("Content-Encoding") != "gzip" {
					t.Error("expected gzip encoding")
				}

				// Verify we can decompress the response
				reader, err := gzip.NewReader(w.Body)
				if err != nil {
					t.Fatalf("failed to create gzip reader: %v", err)
				}
				defer reader.Close()

				_, err = io.ReadAll(reader)
				if err != nil {
					t.Errorf("failed to read gzipped response: %v", err)
				}
			} else {
				if w.Header().Get("Content-Encoding") == "gzip" {
					t.Error("unexpected gzip encoding")
				}
			}

			// Check security headers
			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Error("missing X-Content-Type-Options header")
			}
			if w.Header().Get("X-Frame-Options") != "DENY" {
				t.Error("missing X-Frame-Options header")
			}
		})
	}
}

func TestPrometheusRegistry_HTTPHandler_Caching(t *testing.T) {
	r := NewPrometheusRegistry("test")

	// First call creates the handler
	handler1 := r.HTTPHandler()
	if handler1 == nil {
		t.Fatal("expected non-nil handler")
	}

	// Second call should return the cached handler
	handler2 := r.HTTPHandler()
	if handler2 == nil {
		t.Fatal("expected non-nil handler")
	}

	// Handlers should be the same instance (cached) - we compare by making requests
	req1 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w1 := httptest.NewRecorder()
	handler1.ServeHTTP(w1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w2 := httptest.NewRecorder()
	handler2.ServeHTTP(w2, req2)

	if w1.Code != w2.Code {
		t.Error("expected same behavior from cached handler")
	}

	// Adding a new metric should invalidate the cache
	_, err := r.NewCounter("new_counter", nil)
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}

	// Handler should be recreated after invalidation - verify by checking metrics content
	handler3 := r.HTTPHandler()
	if handler3 == nil {
		t.Fatal("expected non-nil handler after invalidation")
	}

	req3 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w3 := httptest.NewRecorder()
	handler3.ServeHTTP(w3, req3)

	// Should contain the new metric
	if !strings.Contains(w3.Body.String(), "test_new_counter") {
		t.Error("expected new metric to be present after cache invalidation")
	}
}

func TestPrometheusRegistry_HTTPMiddleware(t *testing.T) {
	r := NewPrometheusRegistry("test")

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	tests := []struct {
		name        string
		registry    *PrometheusRegistry
		handlerName string
		handler     http.Handler
		expectSame  bool
	}{
		{
			name:        "valid middleware",
			registry:    r,
			handlerName: "test_handler",
			handler:     testHandler,
			expectSame:  false,
		},
		{
			name:        "nil registry",
			registry:    nil,
			handlerName: "test_handler",
			handler:     testHandler,
			expectSame:  true,
		},
		{
			name:        "nil prometheus registry",
			registry:    &PrometheusRegistry{},
			handlerName: "test_handler",
			handler:     testHandler,
			expectSame:  true,
		},
		{
			name:        "nil handler",
			registry:    r,
			handlerName: "test_handler",
			handler:     nil,
			expectSame:  true,
		},
		{
			name:        "empty handler name",
			registry:    r,
			handlerName: "",
			handler:     testHandler,
			expectSame:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := tt.registry.HTTPMiddleware(tt.handlerName, tt.handler)

			if tt.expectSame {
				// For cases where middleware should not be applied,
				// verify by testing the wrapped handler behavior
				if wrapped == nil && tt.handler != nil {
					t.Error("expected same handler when middleware should not be applied, got nil")
				}
			} else {
				// For cases where middleware should be applied,
				// verify that we get a non-nil wrapped handler
				if wrapped == nil {
					t.Error("expected wrapped handler when middleware should be applied, got nil")
				}
			}
		})
	}
}

func TestPrometheusRegistry_HTTPMiddleware_Metrics(t *testing.T) {
	r := NewPrometheusRegistry("test")

	// Create a test handler that takes some time
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	wrapped := r.HTTPMiddleware("test_handler", testHandler)

	// Make several requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	}

	// Check that HTTP metrics were created
	metrics, err := r.reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	expectedMetrics := []string{
		"test_http_requests_total",
		"test_http_request_duration_seconds",
		"test_http_request_size_bytes",
		"test_http_response_size_bytes",
		"test_http_in_flight_requests",
	}

	for _, expectedMetric := range expectedMetrics {
		found := false
		for _, mf := range metrics {
			if mf.GetName() == expectedMetric {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected metric %s not found", expectedMetric)
		}
	}
}

func TestPrometheusRegistry_HTTPMiddleware_Concurrent(t *testing.T) {
	r := NewPrometheusRegistry("test")

	// Create a test handler that simulates some work
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("concurrent test"))
	})

	wrapped := r.HTTPMiddleware("concurrent_handler", testHandler)

	// Make concurrent requests
	const numRequests = 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			wrapped.ServeHTTP(w, req)
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}

	// Verify metrics were collected
	metrics, err := r.reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	// Should have HTTP metrics
	found := false
	for _, mf := range metrics {
		if mf.GetName() == "test_http_requests_total" {
			found = true
			break
		}
	}
	if !found {
		t.Error("HTTP requests metric not found after concurrent requests")
	}
}

func TestGzipResponseWriter(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	recorder := httptest.NewRecorder()
	grw := &gzipResponseWriter{
		ResponseWriter: recorder,
		gw:             gw,
	}

	// Test Write
	testData := []byte("test data for compression")
	n, err := grw.Write(testData)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != len(testData) {
		t.Errorf("expected %d bytes written, got %d", len(testData), n)
	}

	// Test Flush
	grw.Flush()

	// Test Close
	err = grw.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPrometheusRegistry_InvalidateHandler(t *testing.T) {
	r := NewPrometheusRegistry("test")

	// Get initial handler
	handler1 := r.HTTPHandler()
	if handler1 == nil {
		t.Error("expected non-nil handler")
	}

	// Handler should be cached - verify by getting another handler
	handler2 := r.HTTPHandler()
	if handler2 == nil {
		t.Error("expected non-nil cached handler")
	}

	// Invalidate handler manually (simulating metric registration)
	r.invalidateHandler()

	// New handler should be available
	handler3 := r.HTTPHandler()
	if handler3 == nil {
		t.Error("expected non-nil handler after invalidation")
	}
}

func TestConstants(t *testing.T) {
	if HTTPMaxRequestsInFlight != 10 {
		t.Errorf("expected HTTPMaxRequestsInFlight to be 10, got %d", HTTPMaxRequestsInFlight)
	}
	if HTTPTimeoutSeconds != 5 {
		t.Errorf("expected HTTPTimeoutSeconds to be 5, got %d", HTTPTimeoutSeconds)
	}
	if len(DefaultBuckets) == 0 {
		t.Error("DefaultBuckets should not be empty")
	}
}

func TestCounterImpl_EdgeCases(t *testing.T) {
	r := NewPrometheusRegistry("test")
	counter, err := r.NewCounter("edge_test_counter", nil)
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}

	// Test edge values
	testCases := []struct {
		name      string
		value     float64
		expectAdd bool
	}{
		{"zero", 0.0, true},
		{"small positive", 0.000001, true},
		{"large positive", 1e10, true},
		{"negative", -1.0, false},
		{"negative zero", math.Copysign(0, -1), true}, // Proper negative zero
		{"NaN", math.NaN(), false},
		{"positive infinity", math.Inf(1), false},
		{"negative infinity", math.Inf(-1), false},
	}

	initialValue := 0.0
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			beforeValue := initialValue
			counter.Add(tc.value)

			// Get current value by gathering metrics
			metrics, err := r.reg.Gather()
			if err != nil {
				t.Fatalf("failed to gather metrics: %v", err)
			}

			var currentValue float64
			for _, mf := range metrics {
				if mf.GetName() == "test_edge_test_counter" {
					currentValue = mf.GetMetric()[0].GetCounter().GetValue()
					break
				}
			}

			if tc.expectAdd {
				expectedValue := beforeValue + tc.value
				if currentValue != expectedValue {
					t.Errorf("expected value %f, got %f", expectedValue, currentValue)
				}
				initialValue = currentValue
			} else {
				if currentValue != beforeValue {
					t.Errorf("expected value unchanged at %f, got %f", beforeValue, currentValue)
				}
			}
		})
	}
}

func TestGzipResponseWriter_ErrorHandling(t *testing.T) {
	// Test with a failing writer
	failingWriter := &failingResponseWriter{}
	gw := gzip.NewWriter(failingWriter)

	grw := &gzipResponseWriter{
		ResponseWriter: failingWriter,
		gw:             gw,
	}

	// Write should return the error from gzip writer
	_, err := grw.Write([]byte("test"))
	if err == nil {
		t.Error("expected error from failing writer")
	}

	// Close should also handle errors gracefully
	err = grw.Close()
	if err == nil {
		t.Error("expected error from close")
	}
}

// failingResponseWriter is a mock ResponseWriter that always fails
type failingResponseWriter struct{}

func (f *failingResponseWriter) Header() http.Header {
	return make(http.Header)
}

func (f *failingResponseWriter) Write([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func (f *failingResponseWriter) WriteHeader(statusCode int) {}

func TestPrometheusRegistry_HTTPHandler_EdgeCases(t *testing.T) {
	r := NewPrometheusRegistry("test")
	handler := r.HTTPHandler()

	tests := []struct {
		name       string
		method     string
		path       string
		headers    map[string]string
		wantStatus int
	}{
		{
			name:       "HEAD request",
			method:     http.MethodHead,
			path:       "/metrics",
			headers:    nil,
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST request",
			method:     http.MethodPost,
			path:       "/metrics",
			headers:    nil,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "OPTIONS request",
			method:     http.MethodOptions,
			path:       "/metrics",
			headers:    nil,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:   "gzip with quality values",
			method: http.MethodGet,
			path:   "/metrics",
			headers: map[string]string{
				"Accept-Encoding": "gzip;q=1.0, deflate;q=0.8",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "multiple encodings without gzip",
			method: http.MethodGet,
			path:   "/metrics",
			headers: map[string]string{
				"Accept-Encoding": "deflate, br",
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			// Check security headers
			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Error("missing X-Content-Type-Options header")
			}
			if w.Header().Get("X-Frame-Options") != "DENY" {
				t.Error("missing X-Frame-Options header")
			}
			if w.Header().Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
				t.Error("incorrect Cache-Control header")
			}
			if w.Header().Get("Pragma") != "no-cache" {
				t.Error("missing Pragma header")
			}

			if tt.method == http.MethodPost || tt.method == http.MethodOptions {
				if w.Header().Get("Allow") != "GET, HEAD" {
					t.Error("missing or incorrect Allow header")
				}
			}
		})
	}
}

func TestPrometheusRegistry_GzipCompression_ErrorPaths(t *testing.T) {
	r := NewPrometheusRegistry("test")

	// Add a metric to have content to compress
	counter, err := r.NewCounter("test_counter", nil)
	if err != nil {
		t.Fatalf("failed to create counter: %v", err)
	}
	counter.Inc()

	handler := r.HTTPHandler()

	// Test with gzip support
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Test that gzip encoding header is set
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("expected gzip encoding")
	}

	// Test that we have some compressed content
	gzipData := w.Body.Bytes()
	if len(gzipData) == 0 {
		t.Error("expected non-empty gzip data")
	}

	// Basic test that the data appears to be gzip (starts with gzip magic number)
	if len(gzipData) < 2 || gzipData[0] != 0x1f || gzipData[1] != 0x8b {
		t.Error("response does not appear to be gzip compressed")
	}
}

func TestPrometheusRegistry_MemoryUsage(t *testing.T) {
	r := NewPrometheusRegistry("memory_test")

	// Create and use many metrics to test memory behavior
	const numMetrics = 100
	for i := 0; i < numMetrics; i++ {
		counter, err := r.NewCounter(fmt.Sprintf("test_counter_%d", i), nil)
		if err != nil {
			t.Errorf("failed to create counter %d: %v", i, err)
			continue
		}
		counter.Inc()
	}

	// Gather metrics multiple times to ensure stability
	for i := 0; i < 10; i++ {
		metrics, err := r.reg.Gather()
		if err != nil {
			t.Errorf("failed to gather metrics on iteration %d: %v", i, err)
		}

		// Should have our metrics plus standard collectors
		if len(metrics) < numMetrics {
			t.Errorf("expected at least %d metrics, got %d", numMetrics, len(metrics))
		}
	}
}

func TestPrometheusRegistry_ConcurrentMetricCreation(t *testing.T) {
	r := NewPrometheusRegistry("concurrent_test")

	const numGoroutines = 20
	const metricsPerGoroutine = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	// Create metrics concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := 0; j < metricsPerGoroutine; j++ {
				metricName := fmt.Sprintf("concurrent_metric_%d_%d", routineID, j)

				// Try creating different metric types
				switch j % 3 {
				case 0:
					_, err := r.NewCounter(metricName, nil)
					if err != nil {
						errors <- fmt.Errorf("counter creation failed: %w", err)
					}
				case 1:
					_, err := r.NewGauge(metricName, nil)
					if err != nil {
						errors <- fmt.Errorf("gauge creation failed: %w", err)
					}
				case 2:
					_, err := r.NewHistogram(metricName, nil, nil)
					if err != nil {
						errors <- fmt.Errorf("histogram creation failed: %w", err)
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Error(err)
	}

	// Verify metrics were created
	metrics, err := r.reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	expectedMetrics := numGoroutines * metricsPerGoroutine
	// Should have our metrics plus standard collectors (go/process metrics)
	if len(metrics) < expectedMetrics {
		t.Errorf("expected at least %d metrics, got %d", expectedMetrics, len(metrics))
	}
}

// Hot path benchmarks for metrics operations
func BenchmarkPrometheusRegistry_NewCounter(b *testing.B) {
	r := NewPrometheusRegistry("bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := r.NewCounter(fmt.Sprintf("counter_%d", i), nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrometheusRegistry_NewGauge(b *testing.B) {
	r := NewPrometheusRegistry("bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := r.NewGauge(fmt.Sprintf("gauge_%d", i), nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrometheusRegistry_NewHistogram(b *testing.B) {
	r := NewPrometheusRegistry("bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := r.NewHistogram(fmt.Sprintf("histogram_%d", i), nil, DefaultBuckets)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCounterImpl_Inc(b *testing.B) {
	r := NewPrometheusRegistry("bench")
	counter, err := r.NewCounter("test_counter", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		counter.Inc()
	}
}

func BenchmarkCounterImpl_Add(b *testing.B) {
	r := NewPrometheusRegistry("bench")
	counter, err := r.NewCounter("test_counter", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		counter.Add(1.0)
	}
}

func BenchmarkGaugeImpl_Set(b *testing.B) {
	r := NewPrometheusRegistry("bench")
	gauge, err := r.NewGauge("test_gauge", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gauge.Set(float64(i))
	}
}

func BenchmarkGaugeImpl_Inc(b *testing.B) {
	r := NewPrometheusRegistry("bench")
	gauge, err := r.NewGauge("test_gauge", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		gauge.Inc()
	}
}

func BenchmarkHistogramImpl_Observe(b *testing.B) {
	r := NewPrometheusRegistry("bench")
	histogram, err := r.NewHistogram("test_histogram", nil, DefaultBuckets)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		histogram.Observe(float64(i%100) / 10.0)
	}
}

func BenchmarkPrometheusRegistry_HTTPHandler_Creation(b *testing.B) {
	r := NewPrometheusRegistry("bench")

	// Add some metrics first
	for i := 0; i < 10; i++ {
		r.NewCounter(fmt.Sprintf("counter_%d", i), nil)
		r.NewGauge(fmt.Sprintf("gauge_%d", i), nil)
		r.NewHistogram(fmt.Sprintf("histogram_%d", i), nil, DefaultBuckets)
	}

	b.ResetTimer()
	for b.Loop() {
		// Invalidate cache to force recreation
		r.invalidateHandler()
		_ = r.HTTPHandler()
	}
}

func BenchmarkPrometheusRegistry_HTTPHandler_Cached(b *testing.B) {
	r := NewPrometheusRegistry("bench")

	// Add some metrics and create handler once
	for i := 0; i < 10; i++ {
		r.NewCounter(fmt.Sprintf("counter_%d", i), nil)
		r.NewGauge(fmt.Sprintf("gauge_%d", i), nil)
		r.NewHistogram(fmt.Sprintf("histogram_%d", i), nil, DefaultBuckets)
	}
	_ = r.HTTPHandler() // Prime the cache

	b.ResetTimer()
	for b.Loop() {
		_ = r.HTTPHandler()
	}
}

func BenchmarkPrometheusRegistry_ConcurrentCounterOperations(b *testing.B) {
	r := NewPrometheusRegistry("bench")
	counter, err := r.NewCounter("concurrent_counter", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Inc()
		}
	})
}

func BenchmarkPrometheusRegistry_ConcurrentGaugeOperations(b *testing.B) {
	r := NewPrometheusRegistry("bench")
	gauge, err := r.NewGauge("concurrent_gauge", nil)
	if err != nil {
		b.Fatal(err)
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			gauge.Set(float64(i))
			i++
		}
	})
}

func BenchmarkPrometheusRegistry_ConcurrentHistogramOperations(b *testing.B) {
	r := NewPrometheusRegistry("bench")
	histogram, err := r.NewHistogram("concurrent_histogram", nil, DefaultBuckets)
	if err != nil {
		b.Fatal(err)
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			histogram.Observe(float64(i%100) / 10.0)
			i++
		}
	})
}

func BenchmarkGzipResponseWriter_Write(b *testing.B) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	recorder := httptest.NewRecorder()
	grw := &gzipResponseWriter{
		ResponseWriter: recorder,
		gw:             gw,
	}

	testData := []byte("benchmark test data for compression")

	b.ResetTimer()
	for b.Loop() {
		_, _ = grw.Write(testData)
	}

	grw.Close()
}

func BenchmarkPrometheusRegistry_HTTPMiddleware_Wrapping(b *testing.B) {
	r := NewPrometheusRegistry("bench")
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.HTTPMiddleware(fmt.Sprintf("handler_%d", i), testHandler)
	}
}
