package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	})))
}

func TestNewServer(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{
			name:    "valid address",
			addr:    ":8080",
			wantErr: false,
		},
		{
			name:    "valid address with host",
			addr:    "localhost:8080",
			wantErr: false,
		},
		{
			name:    "valid address with IP",
			addr:    "127.0.0.1:8080",
			wantErr: false,
		},
		{
			name:    "invalid address",
			addr:    "invalid:address",
			wantErr: true,
		},
		{
			name:    "empty address",
			addr:    "",
			wantErr: true,
		},
		{
			name:    "invalid port",
			addr:    ":99999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(tt.addr)
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
			if server == nil {
				t.Error("expected non-nil server")
				return
			}
			if server.addr != tt.addr {
				t.Errorf("expected addr %q, got %q", tt.addr, server.addr)
			}
			if server.health == nil {
				t.Error("expected health manager to be initialized")
			}
			if server.metrics == nil {
				t.Error("expected metrics registry to be initialized")
			}
			if server.readiness == nil {
				t.Error("expected readiness manager to be initialized")
			}
			if server.srv == nil {
				t.Error("expected HTTP server to be initialized")
			}

			// Test server configuration
			if server.srv.Addr != tt.addr {
				t.Errorf("expected server addr %q, got %q", tt.addr, server.srv.Addr)
			}
			if server.srv.ReadTimeout != HTTPReadTimeout {
				t.Errorf("expected read timeout %v, got %v", HTTPReadTimeout, server.srv.ReadTimeout)
			}
			if server.srv.WriteTimeout != HTTPWriteTimeout {
				t.Errorf("expected write timeout %v, got %v", HTTPWriteTimeout, server.srv.WriteTimeout)
			}
		})
	}
}

func TestServer_SetReady(t *testing.T) {
	server, err := NewServer(":0")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Test initial state
	if server.readiness.IsReady() {
		t.Error("expected initial ready state to be false")
	}

	// Test setting ready
	server.SetReady(true)
	if !server.readiness.IsReady() {
		t.Error("expected ready state to be true after SetReady(true)")
	}

	// Test setting not ready
	server.SetReady(false)
	if server.readiness.IsReady() {
		t.Error("expected ready state to be false after SetReady(false)")
	}
}

func TestServer_ReadyzHandler(t *testing.T) {
	server, err := NewServer(":0")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	tests := []struct {
		name           string
		method         string
		ready          bool
		expectStatus   int
		expectJSON     bool
		expectHeaders  map[string]string
		checkJSONField func(t *testing.T, data map[string]any)
	}{
		{
			name:         "GET when ready",
			method:       http.MethodGet,
			ready:        true,
			expectStatus: http.StatusOK,
			expectJSON:   true,
			expectHeaders: map[string]string{
				"Content-Type":  "application/json",
				"Cache-Control": "no-store",
				"Pragma":        "no-cache",
			},
			checkJSONField: func(t *testing.T, data map[string]any) {
				if status, ok := data["status"]; !ok || status != "ok" {
					t.Errorf("expected status 'ok', got %v", status)
				}
				if ready, ok := data["ready"]; !ok || ready != true {
					t.Errorf("expected ready true, got %v", ready)
				}
				if _, ok := data["uptime_sec"]; !ok {
					t.Error("expected uptime_sec field")
				}
			},
		},
		{
			name:         "GET when not ready",
			method:       http.MethodGet,
			ready:        false,
			expectStatus: http.StatusServiceUnavailable,
			expectJSON:   true,
			expectHeaders: map[string]string{
				"Content-Type":  "application/json",
				"Cache-Control": "no-store",
				"Pragma":        "no-cache",
			},
			checkJSONField: func(t *testing.T, data map[string]any) {
				if status, ok := data["status"]; !ok || status != "fail" {
					t.Errorf("expected status 'fail', got %v", status)
				}
				if ready, ok := data["ready"]; !ok || ready != false {
					t.Errorf("expected ready false, got %v", ready)
				}
			},
		},
		{
			name:         "HEAD when ready",
			method:       http.MethodHead,
			ready:        true,
			expectStatus: http.StatusOK,
			expectJSON:   false,
		},
		{
			name:         "HEAD when not ready",
			method:       http.MethodHead,
			ready:        false,
			expectStatus: http.StatusServiceUnavailable,
			expectJSON:   false,
		},
		{
			name:         "POST method not allowed",
			method:       http.MethodPost,
			ready:        true,
			expectStatus: http.StatusMethodNotAllowed,
			expectJSON:   false,
			expectHeaders: map[string]string{
				"Allow": "GET, HEAD",
			},
		},
		{
			name:         "PUT method not allowed",
			method:       http.MethodPut,
			ready:        true,
			expectStatus: http.StatusMethodNotAllowed,
			expectJSON:   false,
			expectHeaders: map[string]string{
				"Allow": "GET, HEAD",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server.SetReady(tt.ready)

			req := httptest.NewRequest(tt.method, ReadyzPath, nil)
			w := httptest.NewRecorder()

			server.ReadyzHandler(w, req)

			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}

			// Check headers
			for key, expectedValue := range tt.expectHeaders {
				if got := w.Header().Get(key); got != expectedValue {
					t.Errorf("expected header %s: %s, got %s", key, expectedValue, got)
				}
			}

			// Check JSON response for GET requests
			if tt.expectJSON {
				var data map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
					t.Errorf("failed to decode JSON response: %v", err)
					return
				}
				if tt.checkJSONField != nil {
					tt.checkJSONField(t, data)
				}
			}
		})
	}
}

func TestServer_LivezHandler(t *testing.T) {
	t.Run("no checkers", func(t *testing.T) {
		testLivezHandlerNoCheckers(t)
	})
	t.Run("successful checkers", func(t *testing.T) {
		testLivezHandlerSuccessfulCheckers(t)
	})
	t.Run("failing checker", func(t *testing.T) {
		testLivezHandlerFailingChecker(t)
	})
	t.Run("HEAD request", func(t *testing.T) {
		testLivezHandlerHead(t)
	})
	t.Run("POST method not allowed", func(t *testing.T) {
		testLivezHandlerMethodNotAllowed(t)
	})
}

func testLivezHandlerNoCheckers(t *testing.T) {
	server, err := NewServer(":0")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, LivezPath, nil)
	w := httptest.NewRecorder()

	server.LivezHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	expectedHeaders := map[string]string{
		"Content-Type":  "application/json",
		"Cache-Control": "no-store",
		"Pragma":        "no-cache",
	}
	for key, expectedValue := range expectedHeaders {
		if got := w.Header().Get(key); got != expectedValue {
			t.Errorf("expected header %s: %s, got %s", key, expectedValue, got)
		}
	}

	var data map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Errorf("failed to decode JSON response: %v", err)
		return
	}

	if status, ok := data["status"]; !ok || status != "ok" {
		t.Errorf("expected status 'ok', got %v", status)
	}
	if healthy, ok := data["healthy"]; !ok || healthy != true {
		t.Errorf("expected healthy true, got %v", healthy)
	}
	if _, ok := data["checks"]; ok {
		t.Error("expected no checks field when no checkers")
	}
}

func testLivezHandlerSuccessfulCheckers(t *testing.T) {
	server, err := NewServer(":0")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	err = server.RegisterChecker("success-checker", 1*time.Second, func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("failed to register checker: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, LivezPath, nil)
	w := httptest.NewRecorder()

	server.LivezHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var data map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Errorf("failed to decode JSON response: %v", err)
		return
	}

	if status, ok := data["status"]; !ok || status != "ok" {
		t.Errorf("expected status 'ok', got %v", status)
	}
	if healthy, ok := data["healthy"]; !ok || healthy != true {
		t.Errorf("expected healthy true, got %v", healthy)
	}
	if checks, ok := data["checks"]; !ok {
		t.Error("expected checks field")
	} else {
		checksMap, ok := checks.(map[string]any)
		if !ok {
			t.Error("expected checks to be a map")
		} else if checksMap["success-checker"] != "ok" {
			t.Errorf("expected success-checker to be 'ok', got %v", checksMap["success-checker"])
		}
	}
}

func testLivezHandlerFailingChecker(t *testing.T) {
	server, err := NewServer(":0")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	err = server.RegisterChecker("fail-checker", 1*time.Second, func() error {
		return errors.New("health check failed")
	})
	if err != nil {
		t.Fatalf("failed to register checker: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, LivezPath, nil)
	w := httptest.NewRecorder()

	server.LivezHandler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	var data map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Errorf("failed to decode JSON response: %v", err)
		return
	}

	if status, ok := data["status"]; !ok || status != "fail" {
		t.Errorf("expected status 'fail', got %v", status)
	}
	if healthy, ok := data["healthy"]; !ok || healthy != false {
		t.Errorf("expected healthy false, got %v", healthy)
	}
	if checks, ok := data["checks"]; !ok {
		t.Error("expected checks field")
	} else {
		checksMap, ok := checks.(map[string]any)
		if !ok {
			t.Error("expected checks to be a map")
		} else if checksMap["fail-checker"] != "fail" {
			t.Errorf("expected fail-checker to be 'fail', got %v", checksMap["fail-checker"])
		}
	}
}

func testLivezHandlerHead(t *testing.T) {
	server, err := NewServer(":0")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodHead, LivezPath, nil)
	w := httptest.NewRecorder()

	server.LivezHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func testLivezHandlerMethodNotAllowed(t *testing.T) {
	server, err := NewServer(":0")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, LivezPath, nil)
	w := httptest.NewRecorder()

	server.LivezHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	if got := w.Header().Get("Allow"); got != "GET, HEAD" {
		t.Errorf("expected Allow header 'GET, HEAD', got %s", got)
	}
}

func TestServer_RegisterChecker(t *testing.T) {
	tests := []struct {
		name      string
		checker   string
		interval  time.Duration
		fn        CheckFunc
		wantErr   bool
		duplicate bool
	}{
		{
			name:     "valid checker",
			checker:  "test-checker",
			interval: 1 * time.Second,
			fn:       func() error { return nil },
			wantErr:  false,
		},
		{
			name:      "duplicate checker",
			checker:   "duplicate-test-checker",
			interval:  1 * time.Second,
			fn:        func() error { return nil },
			wantErr:   true,
			duplicate: true,
		},
		{
			name:     "empty name",
			checker:  "",
			interval: 1 * time.Second,
			fn:       func() error { return nil },
			wantErr:  true,
		},
		{
			name:     "nil function",
			checker:  "nil-func-checker",
			interval: 1 * time.Second,
			fn:       nil,
			wantErr:  true,
		},
		{
			name:     "zero interval",
			checker:  "zero-interval-checker",
			interval: 0,
			fn:       func() error { return nil },
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh server for each test to avoid state interference
			server, err := NewServer(":0")
			if err != nil {
				t.Fatalf("failed to create server: %v", err)
			}

			if tt.duplicate {
				// First register a checker with the same name
				err := server.RegisterChecker(tt.checker, tt.interval, tt.fn)
				if err != nil {
					t.Fatalf("failed to register first checker: %v", err)
				}
			}

			err = server.RegisterChecker(tt.checker, tt.interval, tt.fn)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestServer_RegisterMetrics(t *testing.T) {
	server, err := NewServer(":0")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Test counter registration
	counter, err := server.RegisterCounter("test_counter", map[string]string{"label": "value"})
	if err != nil {
		t.Errorf("failed to register counter: %v", err)
	}
	if counter == nil {
		t.Error("expected non-nil counter")
	}

	// Test gauge registration
	gauge, err := server.RegisterGauge("test_gauge", nil)
	if err != nil {
		t.Errorf("failed to register gauge: %v", err)
	}
	if gauge == nil {
		t.Error("expected non-nil gauge")
	}

	// Test histogram registration
	histogram, err := server.RegisterHistogram("test_histogram", nil, []float64{0.1, 0.5, 1.0})
	if err != nil {
		t.Errorf("failed to register histogram: %v", err)
	}
	if histogram == nil {
		t.Error("expected non-nil histogram")
	}

	// Test duplicate registration error
	_, err = server.RegisterCounter("test_counter", nil)
	if err == nil {
		t.Error("expected error for duplicate counter registration")
	}
}

func TestServer_Shutdown(t *testing.T) {
	tests := []struct {
		name    string
		server  *Server
		timeout time.Duration
		wantErr bool
	}{
		{
			name: "successful shutdown",
			server: func() *Server {
				s, _ := NewServer(":0")
				return s
			}(),
			timeout: 5 * time.Second,
			wantErr: false,
		},
		{
			name:    "nil server",
			server:  nil,
			timeout: 5 * time.Second,
			wantErr: true,
		},
		{
			name: "shutdown with context timeout",
			server: func() *Server {
				s, _ := NewServer(":0")
				return s
			}(),
			timeout: 10 * time.Millisecond, // Short but realistic timeout
			wantErr: false,                 // Should still succeed with quick shutdown
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			err := tt.server.Shutdown(ctx)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestServer_ListenAndServe(t *testing.T) {
	// Test that server can start and stop properly
	server, err := NewServer(":0") // Use random port
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Start server in goroutine
	done := make(chan error, 1)
	go func() {
		done <- server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdownErr := server.Shutdown(ctx)
	if shutdownErr != nil {
		t.Errorf("unexpected shutdown error: %v", shutdownErr)
	}

	// Wait for server to stop
	select {
	case serverErr := <-done:
		if serverErr != nil && serverErr != http.ErrServerClosed {
			t.Errorf("unexpected server error: %v", serverErr)
		}
	case <-time.After(1 * time.Second):
		t.Error("server did not stop within timeout")
	}
}

func TestServer_HTTPEndpoints(t *testing.T) {
	// Start a test server
	server, err := NewServer(":0")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Start server on a random port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		server.srv.Serve(listener)
	}()

	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())

	tests := []struct {
		name         string
		path         string
		method       string
		expectStatus int
	}{
		{
			name:         "livez endpoint",
			path:         LivezPath,
			method:       http.MethodGet,
			expectStatus: http.StatusOK,
		},
		{
			name:         "readyz endpoint",
			path:         ReadyzPath,
			method:       http.MethodGet,
			expectStatus: http.StatusServiceUnavailable, // Not ready initially
		},
		{
			name:         "metrics endpoint",
			path:         MetricsPath,
			method:       http.MethodGet,
			expectStatus: http.StatusOK,
		},
		{
			name:         "non-existent endpoint",
			path:         "/nonexistent",
			method:       http.MethodGet,
			expectStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(baseURL + tt.path)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, resp.StatusCode)
			}
		})
	}

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

func TestServer_ConcurrentRequests(t *testing.T) {
	server, err := NewServer(":0")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	const numRequests = 50
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)

	// Make concurrent requests to different endpoints
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			var path string
			switch index % 3 {
			case 0:
				path = LivezPath
			case 1:
				path = ReadyzPath
			case 2:
				path = MetricsPath
			}

			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			switch path {
			case LivezPath:
				server.LivezHandler(w, req)
			case ReadyzPath:
				server.ReadyzHandler(w, req)
			case MetricsPath:
				handler := server.metrics.HTTPHandler()
				handler.ServeHTTP(w, req)
			}

			if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
				errors <- fmt.Errorf("unexpected status code %d for path %s", w.Code, path)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Error(err)
	}
}

func TestServer_Constants(t *testing.T) {
	if DefaultAddr != ":9091" {
		t.Errorf("expected DefaultAddr to be ':9091', got %q", DefaultAddr)
	}
	if LivezPath != "/livez" {
		t.Errorf("expected LivezPath to be '/livez', got %q", LivezPath)
	}
	if ReadyzPath != "/readyz" {
		t.Errorf("expected ReadyzPath to be '/readyz', got %q", ReadyzPath)
	}
	if MetricsPath != "/metrics" {
		t.Errorf("expected MetricsPath to be '/metrics', got %q", MetricsPath)
	}
	if MetricsNamespace != "watchdog" {
		t.Errorf("expected MetricsNamespace to be 'watchdog', got %q", MetricsNamespace)
	}

	// Test HTTP timeout constants
	if HTTPReadTimeout != 5*time.Second {
		t.Errorf("expected HTTPReadTimeout to be 5s, got %v", HTTPReadTimeout)
	}
	if HTTPWriteTimeout != 10*time.Second {
		t.Errorf("expected HTTPWriteTimeout to be 10s, got %v", HTTPWriteTimeout)
	}
	if HTTPIdleTimeout != 60*time.Second {
		t.Errorf("expected HTTPIdleTimeout to be 60s, got %v", HTTPIdleTimeout)
	}
	if HTTPMaxHeaderBytes != 8192 {
		t.Errorf("expected HTTPMaxHeaderBytes to be 8192, got %d", HTTPMaxHeaderBytes)
	}
}

func TestServer_ServerConfiguration(t *testing.T) {
	server, err := NewServer(":8080")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Test server configuration
	if server.srv.ReadTimeout != HTTPReadTimeout {
		t.Errorf("expected ReadTimeout %v, got %v", HTTPReadTimeout, server.srv.ReadTimeout)
	}
	if server.srv.ReadHeaderTimeout != HTTPReadHeaderTimeout {
		t.Errorf("expected ReadHeaderTimeout %v, got %v", HTTPReadHeaderTimeout, server.srv.ReadHeaderTimeout)
	}
	if server.srv.WriteTimeout != HTTPWriteTimeout {
		t.Errorf("expected WriteTimeout %v, got %v", HTTPWriteTimeout, server.srv.WriteTimeout)
	}
	if server.srv.IdleTimeout != HTTPIdleTimeout {
		t.Errorf("expected IdleTimeout %v, got %v", HTTPIdleTimeout, server.srv.IdleTimeout)
	}
	if server.srv.MaxHeaderBytes != HTTPMaxHeaderBytes {
		t.Errorf("expected MaxHeaderBytes %d, got %d", HTTPMaxHeaderBytes, server.srv.MaxHeaderBytes)
	}
	if !server.srv.DisableGeneralOptionsHandler {
		t.Error("expected DisableGeneralOptionsHandler to be true")
	}
}

// Benchmark tests for server operations
func BenchmarkServer_ReadyzHandler(b *testing.B) {
	server, err := NewServer(":0")
	if err != nil {
		b.Fatalf("failed to create server: %v", err)
	}
	server.SetReady(true)

	req := httptest.NewRequest(http.MethodGet, ReadyzPath, nil)

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		server.ReadyzHandler(w, req)
	}
}

func BenchmarkServer_LivezHandler(b *testing.B) {
	server, err := NewServer(":0")
	if err != nil {
		b.Fatalf("failed to create server: %v", err)
	}

	// Register a few checkers
	for i := 0; i < 3; i++ {
		err := server.RegisterChecker(fmt.Sprintf("checker-%d", i), 1*time.Second, func() error {
			return nil
		})
		if err != nil {
			b.Fatalf("failed to register checker: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, LivezPath, nil)

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		server.LivezHandler(w, req)
	}
}

func BenchmarkServer_ConcurrentReadyz(b *testing.B) {
	server, err := NewServer(":0")
	if err != nil {
		b.Fatalf("failed to create server: %v", err)
	}
	server.SetReady(true)

	req := httptest.NewRequest(http.MethodGet, ReadyzPath, nil)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			server.ReadyzHandler(w, req)
		}
	})
}

func BenchmarkServer_ConcurrentLivez(b *testing.B) {
	server, err := NewServer(":0")
	if err != nil {
		b.Fatalf("failed to create server: %v", err)
	}

	// Register checkers
	for i := 0; i < 5; i++ {
		err := server.RegisterChecker(fmt.Sprintf("checker-%d", i), 1*time.Second, func() error {
			return nil
		})
		if err != nil {
			b.Fatalf("failed to register checker: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, LivezPath, nil)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			server.LivezHandler(w, req)
		}
	})
}
