package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// BenchmarkCounter_Inc benchmarks the high-frequency counter increment operation.
func BenchmarkCounter_Inc(b *testing.B) {
	registry := NewPrometheusRegistry("bench")
	counter, err := registry.NewCounter("test_counter", nil)
	if err != nil {
		b.Fatalf("failed to create counter: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Inc()
		}
	})
}

// BenchmarkCounter_Add benchmarks the high-frequency counter add operation.
func BenchmarkCounter_Add(b *testing.B) {
	registry := NewPrometheusRegistry("bench")
	counter, err := registry.NewCounter("test_counter", nil)
	if err != nil {
		b.Fatalf("failed to create counter: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1.0)
		}
	})
}

// BenchmarkGauge_Set benchmarks the high-frequency gauge set operation.
func BenchmarkGauge_Set(b *testing.B) {
	registry := NewPrometheusRegistry("bench")
	gauge, err := registry.NewGauge("test_gauge", nil)
	if err != nil {
		b.Fatalf("failed to create gauge: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		value := 0.0
		for pb.Next() {
			gauge.Set(value)
			value++
		}
	})
}

// BenchmarkGauge_Inc benchmarks the high-frequency gauge increment operation.
func BenchmarkGauge_Inc(b *testing.B) {
	registry := NewPrometheusRegistry("bench")
	gauge, err := registry.NewGauge("test_gauge", nil)
	if err != nil {
		b.Fatalf("failed to create gauge: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			gauge.Inc()
		}
	})
}

// BenchmarkHistogram_Observe benchmarks the high-frequency histogram observe operation.
func BenchmarkHistogram_Observe(b *testing.B) {
	registry := NewPrometheusRegistry("bench")
	histogram, err := registry.NewHistogram("test_histogram", nil, nil)
	if err != nil {
		b.Fatalf("failed to create histogram: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		value := 0.1
		for pb.Next() {
			histogram.Observe(value)
			value += 0.1
			if value > 10.0 {
				value = 0.1
			}
		}
	})
}

// BenchmarkManager_GetHealthStatus benchmarks the health status retrieval operation.
func BenchmarkManager_GetHealthStatus(b *testing.B) {
	manager := NewHealthManager()
	defer func() {
		_ = manager.Stop(context.Background())
	}()

	// Register some checkers to make it realistic
	for i := range 10 {
		name := "checker_" + string(rune('0'+i))
		err := manager.RegisterChecker(name, 1*time.Hour, func(_ context.Context) error {
			return nil
		})
		if err != nil {
			b.Fatalf("failed to register checker %s: %v", name, err)
		}
	}

	// Wait for initial executions
	time.Sleep(50 * time.Millisecond)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = manager.GetHealthStatus()
		}
	})
}

// BenchmarkHTTPHandler_Metrics benchmarks the metrics HTTP endpoint.
func BenchmarkHTTPHandler_Metrics(b *testing.B) {
	registry := NewPrometheusRegistry("bench")

	// Add some metrics to make it realistic
	for i := range 20 {
		name := "metric_" + string(rune('0'+i%10))
		_, _ = registry.NewCounter(name, map[string]string{"id": string(rune('0' + i))})
	}

	handler := registry.HTTPHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
		}
	})
}

// BenchmarkHTTPHandler_MetricsWithGzip benchmarks the metrics HTTP endpoint with gzip compression.
func BenchmarkHTTPHandler_MetricsWithGzip(b *testing.B) {
	registry := NewPrometheusRegistry("bench")

	// Add more metrics to make compression worthwhile
	for i := range 50 {
		name := "metric_" + string(rune('0'+i%10))
		_, _ = registry.NewCounter(name, map[string]string{"id": string(rune('0' + i))})
	}

	handler := registry.HTTPHandler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
		}
	})
}

// BenchmarkHandlers_ReadyzHandler benchmarks the readiness check endpoint.
func BenchmarkHandlers_ReadyzHandler(b *testing.B) {
	coordinator := NewCoordinator("bench")
	handlers := NewHandlers(coordinator)
	coordinator.SetReady(true)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			recorder := httptest.NewRecorder()
			handlers.ReadyzHandler(recorder, req)
		}
	})
}

// BenchmarkHandlers_LivezHandler benchmarks the liveness check endpoint.
func BenchmarkHandlers_LivezHandler(b *testing.B) {
	coordinator := NewCoordinator("bench")
	handlers := NewHandlers(coordinator)

	// Register some checkers
	for i := range 5 {
		name := "checker_" + string(rune('0'+i))
		err := coordinator.RegisterChecker(name, 1*time.Hour, func(_ context.Context) error {
			return nil
		})
		if err != nil {
			b.Fatalf("failed to register checker %s: %v", name, err)
		}
	}

	defer func() {
		_ = coordinator.Shutdown(context.Background())
	}()

	// Wait for initial executions
	time.Sleep(50 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			recorder := httptest.NewRecorder()
			handlers.LivezHandler(recorder, req)
		}
	})
}

// BenchmarkPrometheusRegistry_NewCounter benchmarks counter creation (not runtime critical but good to know).
func BenchmarkPrometheusRegistry_NewCounter(b *testing.B) {
	registry := NewPrometheusRegistry("bench")

	b.ResetTimer()
	for i := range b.N {
		name := "counter_" + string(rune('0'+i%1000)) // Prevent duplicates with modulo
		_, _ = registry.NewCounter(name, nil)
	}
}

// BenchmarkPrometheusRegistry_NewGauge benchmarks gauge creation.
func BenchmarkPrometheusRegistry_NewGauge(b *testing.B) {
	registry := NewPrometheusRegistry("bench")

	b.ResetTimer()
	for i := range b.N {
		name := "gauge_" + string(rune('0'+i%1000))
		_, _ = registry.NewGauge(name, nil)
	}
}

// BenchmarkPrometheusRegistry_NewHistogram benchmarks histogram creation.
func BenchmarkPrometheusRegistry_NewHistogram(b *testing.B) {
	registry := NewPrometheusRegistry("bench")
	buckets := []float64{0.1, 0.5, 1.0, 5.0, 10.0}

	b.ResetTimer()
	for i := range b.N {
		name := "histogram_" + string(rune('0'+i%1000))
		_, _ = registry.NewHistogram(name, nil, buckets)
	}
}

// BenchmarkMetricsManager_HTTPMiddleware benchmarks the HTTP instrumentation middleware.
func BenchmarkMetricsManager_HTTPMiddleware(b *testing.B) {
	manager := NewMetricsManager("bench")

	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	instrumentedHandler := manager.HTTPMiddleware("test_handler", baseHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			recorder := httptest.NewRecorder()
			instrumentedHandler.ServeHTTP(recorder, req)
		}
	})
}

// BenchmarkManager_RegisterChecker benchmarks health checker registration.
func BenchmarkManager_RegisterChecker(b *testing.B) {
	manager := NewHealthManager()
	defer func() {
		_ = manager.Stop(context.Background())
	}()

	checkFunc := func(_ context.Context) error {
		return nil
	}

	b.ResetTimer()
	for i := range b.N {
		name := "checker_" + string(rune('0'+i%1000))
		_ = manager.RegisterChecker(name, 1*time.Hour, checkFunc)
	}
}

// BenchmarkCheckerExecution benchmarks the execution of a single health checker.
func BenchmarkCheckerExecution(b *testing.B) {
	manager := NewHealthManager()
	defer func() {
		_ = manager.Stop(context.Background())
	}()

	// Create a registered checker to get access to its execution
	rc := &registeredChecker{
		fn: func(_ context.Context) error {
			return nil
		},
		interval:  1 * time.Second,
		lastState: "unknown",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			manager.executeChecker("bench_checker", rc)
		}
	})
}

// BenchmarkSetSecurityHeaders benchmarks security header setting.
func BenchmarkSetSecurityHeaders(b *testing.B) {
	registry := NewPrometheusRegistry("bench")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			recorder := httptest.NewRecorder()
			registry.setSecurityHeaders(recorder)
		}
	})
}

// BenchmarkValidateRequest benchmarks HTTP request validation.
func BenchmarkValidateRequest(b *testing.B) {
	registry := NewPrometheusRegistry("bench")
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			recorder := httptest.NewRecorder()
			registry.validateRequest(recorder, req)
		}
	})
}

// BenchmarkUpdateCheckerState benchmarks health checker state updates.
func BenchmarkUpdateCheckerState(b *testing.B) {
	manager := NewHealthManager()
	defer func() {
		_ = manager.Stop(context.Background())
	}()

	// Register a checker to have something to update
	err := manager.RegisterChecker("bench_checker", 1*time.Hour, func(_ context.Context) error {
		return nil
	})
	if err != nil {
		b.Fatalf("failed to register checker: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			manager.updateCheckerState("bench_checker", nil)
		}
	})
}
