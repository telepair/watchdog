package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// BenchmarkErrorWithMetrics benchmarks error handling with metrics.
func BenchmarkErrorWithMetrics(b *testing.B) {
	client := &Client{
		metrics: NewNoopMetrics("benchmark"),
	}

	testErr := errors.New("test error")

	b.ResetTimer()
	for range b.N {
		_ = client.ErrorWithMetrics(testErr)
	}
}

// BenchmarkWrapValidationError benchmarks error wrapping.
func BenchmarkWrapValidationError(b *testing.B) {
	testErr := ErrInvalidSubject

	b.ResetTimer()
	for range b.N {
		_ = WrapValidationError("subject", testErr)
	}
}

// BenchmarkClientCheckState benchmarks client state checking.
func BenchmarkClientCheckState(b *testing.B) {
	client := &Client{
		closed:  false,
		conn:    nil, // Simulate no connection for benchmark
		metrics: NewNoopMetrics("benchmark"),
	}

	b.ResetTimer()
	for range b.N {
		_ = client.CheckClientState()
	}
}

// BenchmarkConfigValidation benchmarks configuration validation.
func BenchmarkConfigValidation(b *testing.B) {
	config := &Config{
		Name:           "benchmark-client",
		URLs:           []string{"nats://localhost:4222"},
		ConnectTimeout: 2 * time.Second,
		ReconnectWait:  2 * time.Second,
	}

	b.ResetTimer()
	for range b.N {
		_ = config.Validate()
	}
}

// BenchmarkBucketConfigValidation benchmarks bucket configuration validation.
func BenchmarkBucketConfigValidation(b *testing.B) {
	config := &BucketConfig{
		Name:         "benchmark-bucket",
		History:      5,
		Replicas:     1,
		OnMemory:     false,
		Compression:  false,
		MaxBytes:     1024 * 1024,
		MaxValueSize: 1024,
		TTL:          time.Hour,
	}

	b.ResetTimer()
	for range b.N {
		_ = config.Validate()
	}
}

// BenchmarkPublishContextValidation benchmarks the validation part of publish.
func BenchmarkPublishContextValidation(b *testing.B) {
	client := &Client{
		closed:  true, // Closed client to stop at validation
		metrics: NewNoopMetrics("benchmark"),
	}

	ctx := context.Background()
	subject := "benchmark.test"
	data := make([]byte, 1024) // 1KB payload

	b.ResetTimer()
	for range b.N {
		_ = client.PublishContext(ctx, subject, data)
	}
}

// BenchmarkSubscribeContextValidation benchmarks the validation part of subscribe.
func BenchmarkSubscribeContextValidation(b *testing.B) {
	client := &Client{
		closed:  true, // Closed client to stop at validation
		metrics: NewNoopMetrics("benchmark"),
	}

	ctx := context.Background()
	subject := "benchmark.test"
	handler := func(_ *nats.Msg) {}

	b.ResetTimer()
	for range b.N {
		_, _ = client.SubscribeContext(ctx, subject, handler)
	}
}

// BenchmarkMemoryAllocations tests memory allocation patterns.
func BenchmarkMemoryAllocations(b *testing.B) {
	b.Run("SubjectValidation", func(b *testing.B) {
		subject := "test.subject.validation"
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_ = ValidateSubject(subject)
		}
	})

	b.Run("ErrorCreation", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_ = WrapValidationError("subject", ErrInvalidSubject)
		}
	})

	b.Run("ConfigValidation", func(b *testing.B) {
		config := DefaultConfig()
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_ = config.Validate()
		}
	})

	b.Run("DefaultConfigCreation", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_ = DefaultConfig()
		}
	})
}

// BenchmarkCreateMetrics benchmarks metrics creation.
func BenchmarkCreateMetrics(b *testing.B) {
	b.Run("WithCollector", func(b *testing.B) {
		collector := &NoopCollector{}
		clientName := "benchmark-client"

		b.ResetTimer()
		for range b.N {
			_ = createMetrics(collector, clientName)
		}
	})

	b.Run("WithoutCollector", func(b *testing.B) {
		clientName := "benchmark-client"

		b.ResetTimer()
		for range b.N {
			_ = createMetrics(nil, clientName)
		}
	})
}

// BenchmarkStringOperations benchmarks string operations commonly used.
func BenchmarkStringOperations(b *testing.B) {
	b.Run("URLJoin", func(b *testing.B) {
		urls := []string{
			"nats://server1:4222",
			"nats://server2:4222",
			"nats://server3:4222",
		}

		b.ResetTimer()
		for range b.N {
			if len(urls) > 1 {
				_ = strings.Join(urls, ",")
			}
		}
	})

	b.Run("MetricName", func(b *testing.B) {
		clientName := "benchmark-client"
		operation := "messages_sent_total"

		b.ResetTimer()
		for range b.N {
			_ = fmt.Sprintf("natsx_client_%s_%s", clientName, operation)
		}
	})
}
