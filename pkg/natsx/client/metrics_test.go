package client

import (
	"testing"
	"time"
)

type mockMetricsCollector struct {
	counters   map[string]float64
	gauges     map[string]float64
	histograms map[string][]float64
}

func newMockMetricsCollector() *mockMetricsCollector {
	return &mockMetricsCollector{
		counters:   make(map[string]float64),
		gauges:     make(map[string]float64),
		histograms: make(map[string][]float64),
	}
}

func (m *mockMetricsCollector) IncCounter(name string) {
	m.counters[name]++
}

func (m *mockMetricsCollector) AddCounter(name string, value float64) {
	m.counters[name] += value
}

func (m *mockMetricsCollector) SetGauge(name string, value float64) {
	m.gauges[name] = value
}

func (m *mockMetricsCollector) RecordHistogram(name string, value float64) {
	m.histograms[name] = append(m.histograms[name], value)
}

func TestNewMetrics(t *testing.T) {
	collector := newMockMetricsCollector()
	clientName := "test-client"

	metrics := NewMetrics(collector, clientName)

	if metrics == nil {
		t.Fatal("NewMetrics() returned nil")
	}

	if metrics.collector != collector {
		t.Error("collector not set correctly")
	}

	if metrics.clientName != clientName {
		t.Error("client name not set correctly")
	}
}

func TestNewNoopMetrics(t *testing.T) {
	clientName := "test-client"
	metrics := NewNoopMetrics(clientName)

	if metrics == nil {
		t.Fatal("NewNoopMetrics() returned nil")
	}

	if metrics.clientName != clientName {
		t.Error("client name not set correctly")
	}

	// Verify it uses NoopCollector
	if _, ok := metrics.collector.(*NoopCollector); !ok {
		t.Error("expected NoopCollector")
	}
}

func TestMetrics_RecordConnection(t *testing.T) {
	collector := newMockMetricsCollector()
	metrics := NewMetrics(collector, "test-client")

	metrics.RecordConnection()

	// Check external collector calls
	expectedCounter := "natsx_client_test-client_connection_total"
	if collector.counters[expectedCounter] != 1 {
		t.Errorf("expected counter %s to be 1, got %f", expectedCounter, collector.counters[expectedCounter])
	}

	expectedGauge := "natsx_client_test-client_connected"
	if collector.gauges[expectedGauge] != 1 {
		t.Errorf("expected gauge %s to be 1, got %f", expectedGauge, collector.gauges[expectedGauge])
	}

	// Check internal atomic counter
	if metrics.connectionsTotal.Load() != 1 {
		t.Errorf("expected internal connections total to be 1, got %d", metrics.connectionsTotal.Load())
	}
}

func TestMetrics_RecordDisconnection(t *testing.T) {
	collector := newMockMetricsCollector()
	metrics := NewMetrics(collector, "test-client")

	metrics.RecordDisconnection()

	expectedCounter := "natsx_client_test-client_disconnection_total"
	if collector.counters[expectedCounter] != 1 {
		t.Errorf("expected counter %s to be 1, got %f", expectedCounter, collector.counters[expectedCounter])
	}

	expectedGauge := "natsx_client_test-client_connected"
	if collector.gauges[expectedGauge] != 0 {
		t.Errorf("expected gauge %s to be 0, got %f", expectedGauge, collector.gauges[expectedGauge])
	}
}

func TestMetrics_RecordReconnection(t *testing.T) {
	collector := newMockMetricsCollector()
	metrics := NewMetrics(collector, "test-client")

	metrics.RecordReconnection()

	expectedCounter := "natsx_client_test-client_reconnection_total"
	if collector.counters[expectedCounter] != 1 {
		t.Errorf("expected counter %s to be 1, got %f", expectedCounter, collector.counters[expectedCounter])
	}

	expectedGauge := "natsx_client_test-client_connected"
	if collector.gauges[expectedGauge] != 1 {
		t.Errorf("expected gauge %s to be 1, got %f", expectedGauge, collector.gauges[expectedGauge])
	}
}

func TestMetrics_RecordConnectionClosed(t *testing.T) {
	collector := newMockMetricsCollector()
	metrics := NewMetrics(collector, "test-client")

	metrics.RecordConnectionClosed()

	expectedGauge := "natsx_client_test-client_connected"
	if collector.gauges[expectedGauge] != 0 {
		t.Errorf("expected gauge %s to be 0, got %f", expectedGauge, collector.gauges[expectedGauge])
	}
}

func TestMetrics_RecordPublish(t *testing.T) {
	collector := newMockMetricsCollector()
	metrics := NewMetrics(collector, "test-client")

	dataSize := 100
	duration := 50 * time.Millisecond

	metrics.RecordPublish(dataSize, duration)

	// Check counters
	expectedMsgCounter := "natsx_client_test-client_messages_sent_total"
	if collector.counters[expectedMsgCounter] != 1 {
		t.Errorf("expected counter %s to be 1, got %f", expectedMsgCounter, collector.counters[expectedMsgCounter])
	}

	expectedBytesCounter := "natsx_client_test-client_bytes_written_total"
	if collector.counters[expectedBytesCounter] != float64(dataSize) {
		t.Errorf("expected counter %s to be %d, got %f",
			expectedBytesCounter, dataSize, collector.counters[expectedBytesCounter])
	}

	// Check histogram
	expectedHistogram := "natsx_client_test-client_publish_duration_seconds"
	if len(collector.histograms[expectedHistogram]) != 1 {
		t.Errorf("expected 1 histogram entry, got %d", len(collector.histograms[expectedHistogram]))
	}

	// Check internal counters
	if metrics.messagesTotal.Load() != 1 {
		t.Errorf("expected internal messages total to be 1, got %d", metrics.messagesTotal.Load())
	}

	if metrics.bytesTotal.Load() != uint64(dataSize) {
		t.Errorf("expected internal bytes total to be %d, got %d", dataSize, metrics.bytesTotal.Load())
	}
}

func TestMetrics_RecordReceive(t *testing.T) {
	collector := newMockMetricsCollector()
	metrics := NewMetrics(collector, "test-client")

	dataSize := 200

	metrics.RecordReceive(dataSize)

	expectedMsgCounter := "natsx_client_test-client_messages_received_total"
	if collector.counters[expectedMsgCounter] != 1 {
		t.Errorf("expected counter %s to be 1, got %f", expectedMsgCounter, collector.counters[expectedMsgCounter])
	}

	expectedBytesCounter := "natsx_client_test-client_bytes_read_total"
	if collector.counters[expectedBytesCounter] != float64(dataSize) {
		t.Errorf("expected counter %s to be %d, got %f",
			expectedBytesCounter, dataSize, collector.counters[expectedBytesCounter])
	}

	// Check internal counters
	if metrics.messagesTotal.Load() != 1 {
		t.Errorf("expected internal messages total to be 1, got %d", metrics.messagesTotal.Load())
	}

	if metrics.bytesTotal.Load() != uint64(dataSize) {
		t.Errorf("expected internal bytes total to be %d, got %d", dataSize, metrics.bytesTotal.Load())
	}
}

func TestMetrics_RecordError(t *testing.T) {
	collector := newMockMetricsCollector()
	metrics := NewMetrics(collector, "test-client")

	metrics.RecordError()

	expectedCounter := "natsx_client_test-client_errors_total"
	if collector.counters[expectedCounter] != 1 {
		t.Errorf("expected counter %s to be 1, got %f", expectedCounter, collector.counters[expectedCounter])
	}

	if metrics.errorsTotal.Load() != 1 {
		t.Errorf("expected internal errors total to be 1, got %d", metrics.errorsTotal.Load())
	}
}

func TestMetrics_SetSubscriptionCount(t *testing.T) {
	collector := newMockMetricsCollector()
	metrics := NewMetrics(collector, "test-client")

	count := 5
	metrics.SetSubscriptionCount(count)

	expectedGauge := "natsx_client_test-client_subscriptions_active"
	if collector.gauges[expectedGauge] != float64(count) {
		t.Errorf("expected gauge %s to be %d, got %f", expectedGauge, count, collector.gauges[expectedGauge])
	}

	if metrics.subscriptionsActive.Load() != int64(count) {
		t.Errorf("expected internal subscriptions active to be %d, got %d", count, metrics.subscriptionsActive.Load())
	}
}

func TestMetrics_RecordOperation(t *testing.T) {
	collector := newMockMetricsCollector()
	metrics := NewMetrics(collector, "test-client")

	operation := "test_op"
	duration := 100 * time.Millisecond
	success := true

	metrics.RecordOperation(operation, duration, success)

	expectedCounter := "natsx_client_test-client_operations_total_test_op"
	if collector.counters[expectedCounter] != 1 {
		t.Errorf("expected counter %s to be 1, got %f", expectedCounter, collector.counters[expectedCounter])
	}

	expectedHistogram := "natsx_client_test-client_operation_duration_seconds_test_op"
	if len(collector.histograms[expectedHistogram]) != 1 {
		t.Errorf("expected 1 histogram entry, got %d", len(collector.histograms[expectedHistogram]))
	}

	// Test with failure
	metrics.RecordOperation(operation, duration, false)

	expectedErrorCounter := "natsx_client_test-client_operation_errors_total_test_op"
	if collector.counters[expectedErrorCounter] != 1 {
		t.Errorf("expected error counter %s to be 1, got %f",
			expectedErrorCounter, collector.counters[expectedErrorCounter])
	}
}

func TestMetrics_GetStats(t *testing.T) {
	collector := newMockMetricsCollector()
	metrics := NewMetrics(collector, "test-client")

	// Record some metrics
	metrics.RecordConnection()
	metrics.RecordPublish(100, 50*time.Millisecond)
	metrics.RecordReceive(200)
	metrics.RecordError()
	metrics.SetSubscriptionCount(3)

	stats := metrics.GetStats()

	if stats.ConnectionsTotal != 1 {
		t.Errorf("expected connections total 1, got %d", stats.ConnectionsTotal)
	}

	if stats.MessagesTotal != 2 { // 1 publish + 1 receive
		t.Errorf("expected messages total 2, got %d", stats.MessagesTotal)
	}

	if stats.BytesTotal != 300 { // 100 + 200
		t.Errorf("expected bytes total 300, got %d", stats.BytesTotal)
	}

	if stats.ErrorsTotal != 1 {
		t.Errorf("expected errors total 1, got %d", stats.ErrorsTotal)
	}

	if stats.SubscriptionsActive != 3 {
		t.Errorf("expected subscriptions active 3, got %d", stats.SubscriptionsActive)
	}
}

func TestNoopCollector(_ *testing.T) {
	collector := &NoopCollector{}

	// These should not panic
	collector.IncCounter("test")
	collector.AddCounter("test", 1.0)
	collector.SetGauge("test", 1.0)
	collector.RecordHistogram("test", 1.0)
}

func TestPrometheusCollector(t *testing.T) {
	collector := NewPrometheusCollector()

	if collector == nil {
		t.Fatal("NewPrometheusCollector() returned nil")
	}

	if collector.counters == nil {
		t.Error("counters map not initialized")
	}

	if collector.gauges == nil {
		t.Error("gauges map not initialized")
	}

	if collector.histograms == nil {
		t.Error("histograms map not initialized")
	}
}

func TestPrometheusCollector_WithoutRegisteredMetrics(_ *testing.T) {
	collector := NewPrometheusCollector()

	// These should not panic when no metrics are registered
	collector.IncCounter("test")
	collector.AddCounter("test", 1.0)
	collector.SetGauge("test", 1.0)
	collector.RecordHistogram("test", 1.0)
}

func BenchmarkMetrics_RecordPublish(b *testing.B) {
	collector := newMockMetricsCollector()
	metrics := NewMetrics(collector, "test-client")
	duration := 50 * time.Millisecond
	dataSize := 100

	b.ResetTimer()
	for range b.N {
		metrics.RecordPublish(dataSize, duration)
	}
}

func BenchmarkMetrics_GetStats(b *testing.B) {
	collector := newMockMetricsCollector()
	metrics := NewMetrics(collector, "test-client")

	b.ResetTimer()
	for range b.N {
		_ = metrics.GetStats()
	}
}

func BenchmarkNoopCollector(b *testing.B) {
	collector := &NoopCollector{}

	b.ResetTimer()
	for range b.N {
		collector.IncCounter("test")
		collector.AddCounter("test", 1.0)
		collector.SetGauge("test", 1.0)
		collector.RecordHistogram("test", 1.0)
	}
}
