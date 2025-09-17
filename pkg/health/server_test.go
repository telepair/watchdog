package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestMain(_ *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(httptest.NewRecorder(), &slog.HandlerOptions{
		Level: slog.LevelError,
	})))
}

func TestNewHTTPServer(t *testing.T) {
	t.Parallel()

	addr := ":8080"
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := NewHTTPServer(addr, handler)

	if server == nil {
		t.Fatal("NewHTTPServer returned nil")
	}

	if server.server == nil {
		t.Error("HTTP server not initialized")
	}

	if server.server.Addr != addr {
		t.Errorf("expected addr %s, got %s", addr, server.server.Addr)
	}

	if server.logger == nil {
		t.Error("logger not initialized")
	}

	// Check server configuration
	if server.server.ReadTimeout != HTTPReadTimeout {
		t.Errorf("expected read timeout %v, got %v", HTTPReadTimeout, server.server.ReadTimeout)
	}

	if server.server.WriteTimeout != HTTPWriteTimeout {
		t.Errorf("expected write timeout %v, got %v", HTTPWriteTimeout, server.server.WriteTimeout)
	}
}

func TestHTTPServer_Shutdown(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := NewHTTPServer(":0", handler) // Use port 0 for automatic assignment

	// Test shutdown of unstarted server
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestHTTPServer_ShutdownNil(t *testing.T) {
	t.Parallel()

	var server *HTTPServer

	ctx := context.Background()
	err := server.Shutdown(ctx)
	if err == nil {
		t.Error("expected error for nil server shutdown")
	}
}

func TestNewCoordinator(t *testing.T) {
	t.Parallel()

	namespace := "test_coordinator"
	coordinator := NewCoordinator(namespace)

	if coordinator == nil {
		t.Fatal("NewCoordinator returned nil")
	}

	if coordinator.health == nil {
		t.Error("health manager not initialized")
	}

	if coordinator.metrics == nil {
		t.Error("metrics manager not initialized")
	}

	if coordinator.readiness == nil {
		t.Error("readiness manager not initialized")
	}

	if coordinator.logger == nil {
		t.Error("logger not initialized")
	}
}

func TestCoordinator_SetReady(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator("test")

	// Initially should not be ready
	if coordinator.readiness.IsReady() {
		t.Error("coordinator should not be ready initially")
	}

	// Set ready to true
	coordinator.SetReady(true)
	if !coordinator.readiness.IsReady() {
		t.Error("coordinator should be ready after SetReady(true)")
	}

	// Set ready to false
	coordinator.SetReady(false)
	if coordinator.readiness.IsReady() {
		t.Error("coordinator should not be ready after SetReady(false)")
	}
}

func TestCoordinator_RegisterChecker(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator("test")
	defer func() {
		_ = coordinator.Shutdown(context.Background())
	}()

	checkerName := "test_checker"
	interval := 1 * time.Second
	fn := func(_ context.Context) error {
		return nil
	}

	err := coordinator.RegisterChecker(checkerName, interval, fn)
	if err != nil {
		t.Errorf("RegisterChecker() error = %v", err)
	}

	// Verify checker is registered
	status, _ := coordinator.health.GetHealthStatus()
	if _, exists := status[checkerName]; !exists {
		t.Error("checker was not registered")
	}
}

func TestCoordinator_RegisterMetrics(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator("test")
	labels := map[string]string{"service": "test"}

	// Test RegisterCounter
	counter, err := coordinator.RegisterCounter("test_counter", labels)
	if err != nil {
		t.Errorf("RegisterCounter() error = %v", err)
	}
	if counter == nil {
		t.Error("expected non-nil counter")
	}

	// Test RegisterGauge
	gauge, err := coordinator.RegisterGauge("test_gauge", labels)
	if err != nil {
		t.Errorf("RegisterGauge() error = %v", err)
	}
	if gauge == nil {
		t.Error("expected non-nil gauge")
	}

	// Test RegisterHistogram
	buckets := []float64{0.1, 1.0, 10.0}
	histogram, err := coordinator.RegisterHistogram("test_histogram", labels, buckets)
	if err != nil {
		t.Errorf("RegisterHistogram() error = %v", err)
	}
	if histogram == nil {
		t.Error("expected non-nil histogram")
	}
}

func TestCoordinator_Shutdown(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator("test")

	// Register a checker
	err := coordinator.RegisterChecker("test", 1*time.Second, func(_ context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("failed to register checker: %v", err)
	}

	coordinator.SetReady(true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = coordinator.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}

	// Verify readiness is false after shutdown
	if coordinator.readiness.IsReady() {
		t.Error("coordinator should not be ready after shutdown")
	}
}

func TestNewHandlers(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator("test")
	handlers := NewHandlers(coordinator)

	if handlers == nil {
		t.Fatal("NewHandlers returned nil")
	}

	if handlers.coordinator != coordinator {
		t.Error("coordinator not properly set")
	}

	if handlers.logger == nil {
		t.Error("logger not initialized")
	}
}

func TestHandlers_ReadyzHandler(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator("test")
	handlers := NewHandlers(coordinator)

	tests := []struct {
		name           string
		method         string
		ready          bool
		expectedStatus int
		expectedReady  bool
	}{
		{
			name:           "GET ready",
			method:         http.MethodGet,
			ready:          true,
			expectedStatus: http.StatusOK,
			expectedReady:  true,
		},
		{
			name:           "GET not ready",
			method:         http.MethodGet,
			ready:          false,
			expectedStatus: http.StatusServiceUnavailable,
			expectedReady:  false,
		},
		{
			name:           "HEAD ready",
			method:         http.MethodHead,
			ready:          true,
			expectedStatus: http.StatusOK,
			expectedReady:  true,
		},
		{
			name:           "HEAD not ready",
			method:         http.MethodHead,
			ready:          false,
			expectedStatus: http.StatusServiceUnavailable,
			expectedReady:  false,
		},
		{
			name:           "POST not allowed",
			method:         http.MethodPost,
			ready:          true,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedReady:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coordinator.SetReady(tt.ready)

			req := httptest.NewRequest(tt.method, "/readyz", nil)
			recorder := httptest.NewRecorder()

			handlers.ReadyzHandler(recorder, req)

			if recorder.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, recorder.Code)
			}

			if tt.method == http.MethodHead {
				return // HEAD requests don't have body
			}

			if tt.expectedStatus == http.StatusMethodNotAllowed {
				if allow := recorder.Header().Get("Allow"); allow != "GET, HEAD" {
					t.Errorf("expected Allow header 'GET, HEAD', got %q", allow)
				}
				return
			}

			// Check response body for GET requests
			var response map[string]any
			unmarshalErr := json.Unmarshal(recorder.Body.Bytes(), &response)
			if unmarshalErr != nil {
				t.Fatalf("failed to unmarshal response: %v", unmarshalErr)
			}

			if ready, ok := response["ready"].(bool); !ok || ready != tt.expectedReady {
				t.Errorf("expected ready %v, got %v", tt.expectedReady, ready)
			}

			expectedStatus := healthStatusOK
			if !tt.expectedReady {
				expectedStatus = healthStatusFail
			}
			if status, ok := response["status"].(string); !ok || status != expectedStatus {
				t.Errorf("expected status %s, got %s", expectedStatus, status)
			}
		})
	}
}

func TestHandlers_LivezHandler(t *testing.T) {
	t.Parallel()

	coordinator := NewCoordinator("test")
	handlers := NewHandlers(coordinator)

	// Register a passing checker
	err := coordinator.RegisterChecker("pass", 100*time.Millisecond, func(_ context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("failed to register passing checker: %v", err)
	}

	// Register a failing checker
	err = coordinator.RegisterChecker("fail", 100*time.Millisecond, func(_ context.Context) error {
		return errors.New("test failure")
	})
	if err != nil {
		t.Fatalf("failed to register failing checker: %v", err)
	}

	defer func() {
		_ = coordinator.Shutdown(context.Background())
	}()

	// Wait for checkers to run
	time.Sleep(200 * time.Millisecond)

	tests := []struct {
		name            string
		method          string
		expectedStatus  int
		expectedHealthy bool
	}{
		{
			name:            "GET with failures",
			method:          http.MethodGet,
			expectedStatus:  http.StatusServiceUnavailable,
			expectedHealthy: false,
		},
		{
			name:            "HEAD with failures",
			method:          http.MethodHead,
			expectedStatus:  http.StatusServiceUnavailable,
			expectedHealthy: false,
		},
		{
			name:            "POST not allowed",
			method:          http.MethodPost,
			expectedStatus:  http.StatusMethodNotAllowed,
			expectedHealthy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/livez", nil)
			recorder := httptest.NewRecorder()

			handlers.LivezHandler(recorder, req)

			if recorder.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, recorder.Code)
			}

			if tt.method == http.MethodHead {
				return // HEAD requests don't have body
			}

			if tt.expectedStatus == http.StatusMethodNotAllowed {
				if allow := recorder.Header().Get("Allow"); allow != "GET, HEAD" {
					t.Errorf("expected Allow header 'GET, HEAD', got %q", allow)
				}
				return
			}

			// Check response body for GET requests
			var response map[string]any
			unmarshalErr := json.Unmarshal(recorder.Body.Bytes(), &response)
			if unmarshalErr != nil {
				t.Fatalf("failed to unmarshal response: %v", unmarshalErr)
			}

			if healthy, ok := response["healthy"].(bool); !ok || healthy != tt.expectedHealthy {
				t.Errorf("expected healthy %v, got %v", tt.expectedHealthy, healthy)
			}

			if checks, ok := response["checks"].(map[string]any); ok {
				if _, hasPass := checks["pass"]; !hasPass {
					t.Error("expected 'pass' checker in response")
				}
				if _, hasFail := checks["fail"]; !hasFail {
					t.Error("expected 'fail' checker in response")
				}
			} else {
				t.Error("expected 'checks' field in response")
			}
		})
	}
}

func TestNewServer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Addr:             ":8080",
				LivezPath:        "/livez",
				ReadyzPath:       "/readyz",
				MetricsPath:      "/metrics",
				MetricsNamespace: "test",
			},
			wantErr: false,
		},
		{
			name: "invalid config",
			cfg: Config{
				Addr:        "invalid-address-format", // Invalid address format
				LivezPath:   "/livez",
				ReadyzPath:  "/readyz",
				MetricsPath: "/metrics",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewServer() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if server == nil {
					t.Error("expected non-nil server")
				} else {
					if server.coordinator == nil {
						t.Error("coordinator not initialized")
					}
					if server.handlers == nil {
						t.Error("handlers not initialized")
					}
					if server.httpServer == nil {
						t.Error("HTTP server not initialized")
					}
				}
			}
		})
	}
}

func TestServer_Methods(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Addr:             ":0",
		LivezPath:        "/livez",
		ReadyzPath:       "/readyz",
		MetricsPath:      "/metrics",
		MetricsNamespace: "test",
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Test SetReady
	server.SetReady(true)
	// No direct way to verify this, but it should not panic

	// Test RegisterChecker
	err = server.RegisterChecker("test", 1*time.Second, func(_ context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("RegisterChecker() error = %v", err)
	}

	// Test RegisterCounter
	counter, err := server.RegisterCounter("test_counter", nil)
	if err != nil {
		t.Errorf("RegisterCounter() error = %v", err)
	}
	if counter == nil {
		t.Error("expected non-nil counter")
	}

	// Test RegisterGauge
	gauge, err := server.RegisterGauge("test_gauge", nil)
	if err != nil {
		t.Errorf("RegisterGauge() error = %v", err)
	}
	if gauge == nil {
		t.Error("expected non-nil gauge")
	}

	// Test RegisterHistogram
	histogram, err := server.RegisterHistogram("test_histogram", nil, nil)
	if err != nil {
		t.Errorf("RegisterHistogram() error = %v", err)
	}
	if histogram == nil {
		t.Error("expected non-nil histogram")
	}

	// Test Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestServer_ShutdownNil(t *testing.T) {
	t.Parallel()

	var server *Server
	ctx := context.Background()

	err := server.Shutdown(ctx)
	if err == nil {
		t.Error("expected error for nil server shutdown")
	}
}

func TestServer_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Addr:             ":0",
		LivezPath:        "/livez",
		ReadyzPath:       "/readyz",
		MetricsPath:      "/metrics",
		MetricsNamespace: "test",
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	const numGoroutines = 10
	var wg sync.WaitGroup

	// Concurrently register checkers
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			checkerName := fmt.Sprintf("checker_%d", id)
			checkerErr := server.RegisterChecker(checkerName, 1*time.Second, func(_ context.Context) error {
				return nil
			})
			if checkerErr != nil {
				t.Errorf("failed to register checker %s: %v", checkerName, checkerErr)
			}
		}(i)
	}

	// Concurrently register metrics
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			metricName := fmt.Sprintf("metric_%d", id)
			_, regErr := server.RegisterCounter(metricName, nil)
			if regErr != nil {
				t.Errorf("failed to register counter %s: %v", metricName, regErr)
			}
		}(i)
	}

	// Concurrently set ready state
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			server.SetReady(id%2 == 0)
		}(i)
	}

	wg.Wait()
}

// ExampleNewServer demonstrates how to create and use a health server.
func ExampleNewServer() {
	// Create server configuration
	cfg := Config{
		Addr:             ":8080",
		LivezPath:        "/health/live",
		ReadyzPath:       "/health/ready",
		MetricsPath:      "/metrics",
		MetricsNamespace: "myapp",
	}

	// Create the health server
	server, err := NewServer(cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create server: %v", err))
	}

	// Register a health checker
	err = server.RegisterChecker("database", 30*time.Second, func(_ context.Context) error {
		// Check database connectivity
		// return nil if healthy, error if unhealthy
		return nil
	})
	if err != nil {
		panic(fmt.Sprintf("failed to register checker: %v", err))
	}

	// Register metrics
	requestCounter, err := server.RegisterCounter("requests_total",
		map[string]string{"service": "api"})
	if err != nil {
		panic(fmt.Sprintf("failed to register counter: %v", err))
	}

	responseTimeHistogram, err := server.RegisterHistogram("response_time_seconds",
		map[string]string{"endpoint": "/api"},
		[]float64{0.1, 0.5, 1.0, 5.0})
	if err != nil {
		panic(fmt.Sprintf("failed to register histogram: %v", err))
	}

	// Set service as ready
	server.SetReady(true)

	// Use metrics in your application
	requestCounter.Inc()
	responseTimeHistogram.Observe(0.25)

	// Start the server (in a real application)
	// ctx := context.Background()
	// if err := server.ListenAndServe(ctx); err != nil {
	//     log.Printf("server error: %v", err)
	// }

	// Graceful shutdown (in a real application)
	// ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	// defer cancel()
	// if err := server.Shutdown(ctx); err != nil {
	//     log.Printf("shutdown error: %v", err)
	// }

	fmt.Println("Health server configured successfully")
	// Output: Health server configured successfully
}

// ExampleServer_endpoints demonstrates the available HTTP endpoints.
func ExampleServer_endpoints() {
	cfg := Config{
		Addr:             ":8080",
		LivezPath:        "/livez",
		ReadyzPath:       "/readyz",
		MetricsPath:      "/metrics",
		MetricsNamespace: "example",
	}

	server, _ := NewServer(cfg)
	defer server.Shutdown(context.Background())

	// The server exposes three endpoints:

	// 1. Liveness endpoint - checks if the application is alive
	// GET/HEAD http://localhost:8080/livez
	// Returns 200 if all health checkers pass, 503 if any fail

	// 2. Readiness endpoint - checks if the application is ready to serve traffic
	// GET/HEAD http://localhost:8080/readyz
	// Returns 200 if ready, 503 if not ready

	// 3. Metrics endpoint - exposes Prometheus metrics
	// GET http://localhost:8080/metrics
	// Returns metrics in Prometheus text format

	fmt.Println("Server endpoints configured")
	// Output: Server endpoints configured
}
