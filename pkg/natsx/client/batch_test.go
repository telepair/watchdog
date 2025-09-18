package client

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMessage(t *testing.T) {
	msg := Message{
		Subject: "test.subject",
		Data:    []byte("test data"),
	}

	if msg.Subject != "test.subject" {
		t.Errorf("Expected subject 'test.subject', got %s", msg.Subject)
	}

	if string(msg.Data) != "test data" {
		t.Errorf("Expected data 'test data', got %s", string(msg.Data))
	}
}

func TestPublishBatchEmpty(t *testing.T) {
	config := DefaultConfig()
	config.URLs = []string{"nats://localhost:4222"}

	client, err := NewClient(config)
	if err != nil {
		t.Skip("NATS server not available")
	}
	defer client.Close()

	ctx := context.Background()
	err = client.PublishBatch(ctx, nil)
	if err != nil {
		t.Errorf("PublishBatch with empty slice should not error: %v", err)
	}

	err = client.PublishBatch(ctx, []Message{})
	if err != nil {
		t.Errorf("PublishBatch with empty slice should not error: %v", err)
	}
}

func TestPublishBatchValidation(t *testing.T) {
	config := DefaultConfig()
	config.URLs = []string{"nats://localhost:4222"}

	client, err := NewClient(config)
	if err != nil {
		t.Skip("NATS server not available")
	}
	defer client.Close()

	ctx := context.Background()

	// Test invalid subject
	messages := []Message{
		{Subject: "", Data: []byte("test")},
	}

	err = client.PublishBatch(ctx, messages)
	if err == nil {
		t.Error("Expected error for invalid subject")
	}

	// Test invalid subject with special characters
	messages = []Message{
		{Subject: "invalid subject with spaces", Data: []byte("test")},
	}

	err = client.PublishBatch(ctx, messages)
	if err == nil {
		t.Error("Expected error for invalid subject with spaces")
	}
}

func TestPublishBatchWithFlush(t *testing.T) {
	config := DefaultConfig()
	config.URLs = []string{"nats://localhost:4222"}

	client, err := NewClient(config)
	if err != nil {
		t.Skip("NATS server not available")
	}
	defer client.Close()

	ctx := context.Background()

	messages := []Message{
		{Subject: "test.1", Data: []byte("message 1")},
		{Subject: "test.2", Data: []byte("message 2")},
		{Subject: "test.3", Data: []byte("message 3")},
	}

	// Test with flush enabled
	err = client.PublishBatchWithFlush(ctx, messages, true)
	if err != nil {
		t.Errorf("PublishBatchWithFlush(flush=true) failed: %v", err)
	}

	// Test with flush disabled
	err = client.PublishBatchWithFlush(ctx, messages, false)
	if err != nil {
		t.Errorf("PublishBatchWithFlush(flush=false) failed: %v", err)
	}
}

func TestPublishBatchContext(t *testing.T) {
	config := DefaultConfig()
	config.URLs = []string{"nats://localhost:4222"}

	client, err := NewClient(config)
	if err != nil {
		t.Skip("NATS server not available")
	}
	defer client.Close()

	messages := []Message{
		{Subject: "test.timeout", Data: []byte("test data")},
	}

	// Test with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = client.PublishBatch(ctx, messages)
	if err != nil {
		t.Logf("PublishBatch with timeout: %v", err)
	}

	// Test with cancelled context
	ctx, cancel = context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = client.PublishBatch(ctx, messages)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestPublishBatchLarge(t *testing.T) {
	config := DefaultConfig()
	config.URLs = []string{"nats://localhost:4222"}

	client, err := NewClient(config)
	if err != nil {
		t.Skip("NATS server not available")
	}
	defer client.Close()

	ctx := context.Background()

	// Create a batch of messages
	messages := make([]Message, 100)
	for i := 0; i < 100; i++ {
		messages[i] = Message{
			Subject: fmt.Sprintf("test.batch.%d", i),
			Data:    []byte(fmt.Sprintf("message %d data", i)),
		}
	}

	start := time.Now()
	err = client.PublishBatch(ctx, messages)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("PublishBatch with 100 messages failed: %v", err)
	}

	t.Logf("Published 100 messages in %v", elapsed)

	// Should be faster than individual publishes
	if elapsed > 5*time.Second {
		t.Errorf("Batch publish took too long: %v", elapsed)
	}
}

func BenchmarkPublishBatch(b *testing.B) {
	config := DefaultConfig()
	config.URLs = []string{"nats://localhost:4222"}

	client, err := NewClient(config)
	if err != nil {
		b.Skip("NATS server not available")
	}
	defer client.Close()

	ctx := context.Background()
	messages := []Message{
		{Subject: "bench.test", Data: []byte("benchmark data")},
		{Subject: "bench.test2", Data: []byte("benchmark data 2")},
		{Subject: "bench.test3", Data: []byte("benchmark data 3")},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := client.PublishBatch(ctx, messages)
		if err != nil {
			b.Fatalf("PublishBatch failed: %v", err)
		}
	}
}

func BenchmarkPublishIndividual(b *testing.B) {
	config := DefaultConfig()
	config.URLs = []string{"nats://localhost:4222"}

	client, err := NewClient(config)
	if err != nil {
		b.Skip("NATS server not available")
	}
	defer client.Close()

	ctx := context.Background()
	messages := []Message{
		{Subject: "bench.test", Data: []byte("benchmark data")},
		{Subject: "bench.test2", Data: []byte("benchmark data 2")},
		{Subject: "bench.test3", Data: []byte("benchmark data 3")},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, msg := range messages {
			err := client.PublishContext(ctx, msg.Subject, msg.Data)
			if err != nil {
				b.Fatalf("PublishContext failed: %v", err)
			}
		}
	}
}

func BenchmarkPublishBatchSizes(b *testing.B) {
	config := DefaultConfig()
	config.URLs = []string{"nats://localhost:4222"}

	client, err := NewClient(config)
	if err != nil {
		b.Skip("NATS server not available")
	}
	defer client.Close()

	ctx := context.Background()

	batchSizes := []int{1, 5, 10, 25, 50, 100}

	for _, size := range batchSizes {
		messages := make([]Message, size)
		for i := 0; i < size; i++ {
			messages[i] = Message{
				Subject: fmt.Sprintf("bench.batch.%d", i),
				Data:    []byte(fmt.Sprintf("benchmark data %d", i)),
			}
		}

		b.Run(fmt.Sprintf("BatchSize_%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := client.PublishBatch(ctx, messages)
				if err != nil {
					b.Fatalf("PublishBatch failed: %v", err)
				}
			}
		})
	}
}
