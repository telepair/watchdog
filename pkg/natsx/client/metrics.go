package client

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector defines the interface for metrics collection.
type MetricsCollector interface {
	IncCounter(name string)
	AddCounter(name string, value float64)
	SetGauge(name string, value float64)
	RecordHistogram(name string, value float64)
}

// Metrics holds metrics for NATS client operations.
type Metrics struct {
	collector  MetricsCollector
	clientName string

	// Internal atomic counters for basic stats
	connectionsTotal    atomic.Uint64
	messagesTotal       atomic.Uint64
	bytesTotal          atomic.Uint64
	errorsTotal         atomic.Uint64
	subscriptionsActive atomic.Int64
}

// NewMetrics creates a new client metrics instance.
func NewMetrics(collector MetricsCollector, clientName string) *Metrics {
	return &Metrics{
		collector:  collector,
		clientName: clientName,
	}
}

// NewNoopMetrics creates a client metrics instance without external collector.
func NewNoopMetrics(clientName string) *Metrics {
	return &Metrics{
		collector:  &NoopCollector{},
		clientName: clientName,
	}
}

// RecordConnection records a successful connection.
func (m *Metrics) RecordConnection() {
	if m.collector != nil {
		m.collector.IncCounter(fmt.Sprintf("natsx_client_%s_connection_total", m.clientName))
		m.collector.SetGauge(fmt.Sprintf("natsx_client_%s_connected", m.clientName), 1)
	}
	m.connectionsTotal.Add(1)
}

// RecordDisconnection records a disconnection event.
func (m *Metrics) RecordDisconnection() {
	if m.collector != nil {
		m.collector.IncCounter(fmt.Sprintf("natsx_client_%s_disconnection_total", m.clientName))
		m.collector.SetGauge(fmt.Sprintf("natsx_client_%s_connected", m.clientName), 0)
	}
}

// RecordReconnection records a reconnection event.
func (m *Metrics) RecordReconnection() {
	if m.collector != nil {
		m.collector.IncCounter(fmt.Sprintf("natsx_client_%s_reconnection_total", m.clientName))
		m.collector.SetGauge(fmt.Sprintf("natsx_client_%s_connected", m.clientName), 1)
	}
}

// RecordConnectionClosed records a connection close event.
func (m *Metrics) RecordConnectionClosed() {
	if m.collector != nil {
		m.collector.SetGauge(fmt.Sprintf("natsx_client_%s_connected", m.clientName), 0)
	}
}

// RecordPublish records a message publish operation.
func (m *Metrics) RecordPublish(dataSize int, duration time.Duration) {
	if m.collector != nil {
		m.collector.IncCounter(fmt.Sprintf("natsx_client_%s_messages_sent_total", m.clientName))
		m.collector.AddCounter(fmt.Sprintf("natsx_client_%s_bytes_written_total", m.clientName), float64(dataSize))
		histogramName := fmt.Sprintf("natsx_client_%s_publish_duration_seconds", m.clientName)
		m.collector.RecordHistogram(histogramName, duration.Seconds())
	}
	m.messagesTotal.Add(1)
	if dataSize > 0 {
		m.bytesTotal.Add(uint64(dataSize))
	}
}

// RecordReceive records a message receive operation.
func (m *Metrics) RecordReceive(dataSize int) {
	if m.collector != nil {
		m.collector.IncCounter(fmt.Sprintf("natsx_client_%s_messages_received_total", m.clientName))
		m.collector.AddCounter(fmt.Sprintf("natsx_client_%s_bytes_read_total", m.clientName), float64(dataSize))
	}
	m.messagesTotal.Add(1)
	if dataSize > 0 {
		m.bytesTotal.Add(uint64(dataSize))
	}
}

// RecordError records an error occurrence.
func (m *Metrics) RecordError() {
	if m.collector != nil {
		m.collector.IncCounter(fmt.Sprintf("natsx_client_%s_errors_total", m.clientName))
	}
	m.errorsTotal.Add(1)
}

// SetSubscriptionCount sets the current number of active subscriptions.
func (m *Metrics) SetSubscriptionCount(count int) {
	if m.collector != nil {
		m.collector.SetGauge(fmt.Sprintf("natsx_client_%s_subscriptions_active", m.clientName), float64(count))
	}
	m.subscriptionsActive.Store(int64(count))
}

// RecordOperation records a generic operation with duration.
func (m *Metrics) RecordOperation(operation string, duration time.Duration, success bool) {
	if m.collector != nil {
		counterName := fmt.Sprintf("natsx_client_%s_operations_total_%s", m.clientName, operation)
		histogramName := fmt.Sprintf("natsx_client_%s_operation_duration_seconds_%s", m.clientName, operation)

		m.collector.IncCounter(counterName)
		m.collector.RecordHistogram(histogramName, duration.Seconds())

		if !success {
			errorCounterName := fmt.Sprintf("natsx_client_%s_operation_errors_total_%s", m.clientName, operation)
			m.collector.IncCounter(errorCounterName)
		}
	}
}

// GetStats returns current statistics.
func (m *Metrics) GetStats() Stats {
	return Stats{
		ConnectionsTotal:    m.connectionsTotal.Load(),
		MessagesTotal:       m.messagesTotal.Load(),
		BytesTotal:          m.bytesTotal.Load(),
		ErrorsTotal:         m.errorsTotal.Load(),
		SubscriptionsActive: m.subscriptionsActive.Load(),
	}
}

// Stats represents client statistics.
type Stats struct {
	ConnectionsTotal    uint64 `json:"connections_total"`
	MessagesTotal       uint64 `json:"messages_total"`
	BytesTotal          uint64 `json:"bytes_total"`
	ErrorsTotal         uint64 `json:"errors_total"`
	SubscriptionsActive int64  `json:"subscriptions_active"`
}

// NoopCollector is a no-op collector that does nothing.
type NoopCollector struct{}

func (n *NoopCollector) IncCounter(_ string)                 {}
func (n *NoopCollector) AddCounter(_ string, _ float64)      {}
func (n *NoopCollector) SetGauge(_ string, _ float64)        {}
func (n *NoopCollector) RecordHistogram(_ string, _ float64) {}

// Counter interface for Prometheus metrics.
type Counter interface {
	Inc()
	Add(v float64)
}

// Gauge interface for Prometheus metrics.
type Gauge interface {
	Set(v float64)
	Inc()
	Dec()
	Add(v float64)
}

// Histogram interface for Prometheus metrics.
type Histogram interface {
	Observe(v float64)
}

// PrometheusCollector bridges natsx metrics to Prometheus types.
type PrometheusCollector struct {
	counters   map[string]Counter
	gauges     map[string]Gauge
	histograms map[string]Histogram
	mu         sync.RWMutex
}

// NewPrometheusCollector creates a new Prometheus metrics collector.
func NewPrometheusCollector() *PrometheusCollector {
	return &PrometheusCollector{
		counters:   make(map[string]Counter),
		gauges:     make(map[string]Gauge),
		histograms: make(map[string]Histogram),
	}
}

// RegisterCounter registers a Counter for a given metric name.
func (p *PrometheusCollector) RegisterCounter(name string, counter Counter) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.counters[name] = counter
}

// RegisterGauge registers a Gauge for a given metric name.
func (p *PrometheusCollector) RegisterGauge(name string, gauge Gauge) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gauges[name] = gauge
}

// RegisterHistogram registers a Histogram for a given metric name.
func (p *PrometheusCollector) RegisterHistogram(name string, histogram Histogram) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.histograms[name] = histogram
}

// IncCounter implements MetricsCollector interface.
func (p *PrometheusCollector) IncCounter(name string) {
	p.mu.RLock()
	counter := p.counters[name]
	p.mu.RUnlock()

	if counter != nil {
		counter.Inc()
	}
}

// AddCounter implements MetricsCollector interface.
func (p *PrometheusCollector) AddCounter(name string, value float64) {
	p.mu.RLock()
	counter := p.counters[name]
	p.mu.RUnlock()

	if counter != nil {
		counter.Add(value)
	}
}

// SetGauge implements MetricsCollector interface.
func (p *PrometheusCollector) SetGauge(name string, value float64) {
	p.mu.RLock()
	gauge := p.gauges[name]
	p.mu.RUnlock()

	if gauge != nil {
		gauge.Set(value)
	}
}

// RecordHistogram implements MetricsCollector interface.
func (p *PrometheusCollector) RecordHistogram(name string, value float64) {
	p.mu.RLock()
	histogram := p.histograms[name]
	p.mu.RUnlock()

	if histogram != nil {
		histogram.Observe(value)
	}
}
