package system

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/telepair/watchdog/internal/collector/types"
)

var _ types.Collector = (*Collector)(nil)
var collectorName = "system-metrics"

// metricCollector represents a single metric type collector
type metricCollector struct {
	subject     string
	interval    time.Duration
	collectFunc func(context.Context) (any, error)
	success     atomic.Bool
}

// Collector manages collection and publishing of system metrics
type Collector struct {
	cfg           Config
	subjectPrefix string
	reporter      types.Publisher

	// Lifecycle management
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started atomic.Bool

	// Metric collectors
	metrics []*metricCollector

	logger *slog.Logger
}

// NewCollector creates a new system metrics collector
func NewCollector(cfg *Config, subjectPrefix string, reporter types.Publisher) (*Collector, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cfg is required")
	}
	if reporter == nil {
		return nil, fmt.Errorf("reporter is required")
	}

	if err := cfg.Parse(); err != nil {
		return nil, fmt.Errorf("failed to parse cfg: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &Collector{
		cfg:           *cfg,
		subjectPrefix: strings.TrimRight(subjectPrefix, ".") + ".",
		reporter:      reporter,
		ctx:           ctx,
		cancel:        cancel,
		logger:        slog.Default().With("component", collectorName),
	}

	c.initMetricCollectors()

	return c, nil
}

// initMetricCollectors initializes all metric collectors based on configuration
func (c *Collector) initMetricCollectors() {
	c.metrics = make([]*metricCollector, 0, 6)

	// CPU metrics
	if c.cfg.CPU.IsEnabled() {
		c.metrics = append(c.metrics, &metricCollector{
			subject:     c.cfg.CPU.SubjectSuffix,
			interval:    c.cfg.CPU.GetInterval(),
			collectFunc: func(ctx context.Context) (any, error) { return CollectCPU(ctx) },
		})
	}

	// Memory metrics
	if c.cfg.Memory.IsEnabled() {
		c.metrics = append(c.metrics, &metricCollector{
			subject:     c.cfg.Memory.SubjectSuffix,
			interval:    c.cfg.Memory.GetInterval(),
			collectFunc: func(ctx context.Context) (any, error) { return CollectMemory(ctx) },
		})
	}

	// Disk metrics
	if c.cfg.Disk.IsEnabled() {
		c.metrics = append(c.metrics, &metricCollector{
			subject:     c.cfg.Disk.SubjectSuffix,
			interval:    c.cfg.Disk.GetInterval(),
			collectFunc: func(ctx context.Context) (any, error) { return CollectDisk(ctx) },
		})
	}

	// Network metrics
	if c.cfg.Network.IsEnabled() {
		c.metrics = append(c.metrics, &metricCollector{
			subject:     c.cfg.Network.SubjectSuffix,
			interval:    c.cfg.Network.GetInterval(),
			collectFunc: func(ctx context.Context) (any, error) { return CollectNetwork(ctx) },
		})
	}

	// Load metrics
	if c.cfg.Load.IsEnabled() {
		c.metrics = append(c.metrics, &metricCollector{
			subject:     c.cfg.Load.SubjectSuffix,
			interval:    c.cfg.Load.GetInterval(),
			collectFunc: func(ctx context.Context) (any, error) { return CollectLoad(ctx) },
		})
	}

	// Uptime metrics
	if c.cfg.Uptime.IsEnabled() {
		c.metrics = append(c.metrics, &metricCollector{
			subject:     c.cfg.Uptime.SubjectSuffix,
			interval:    c.cfg.Uptime.GetInterval(),
			collectFunc: func(ctx context.Context) (any, error) { return CollectUptime(ctx) },
		})
	}
}

// Name returns the collector name
func (c *Collector) Name() string {
	return collectorName
}

// Start begins metric collection
func (c *Collector) Start() error {
	if c.started.Load() {
		return fmt.Errorf("collector already started")
	}
	c.logger.Info("starting system collector", "metrics_count", len(c.metrics))

	for _, metric := range c.metrics {
		c.wg.Go(func() {
			c.runMetricCollector(metric)
		})
	}
	c.started.Store(true)
	return nil
}

// Stop halts metric collection
func (c *Collector) Stop() error {
	if !c.started.Load() {
		return nil
	}
	c.started.Store(false)

	c.logger.Info("stopping system collector")

	c.cancel()

	c.wg.Wait()
	c.logger.Info("system collector stopped")
	return nil
}

// Health checks the collector health
func (c *Collector) Health() error {
	if !c.started.Load() {
		return fmt.Errorf("collector not started")
	}

	if err := c.ctx.Err(); err != nil {
		return fmt.Errorf("collector context error: %w", err)
	}

	for _, metric := range c.metrics {
		if !metric.success.Load() {
			return fmt.Errorf("metric collector %s failed", metric.subject)
		}
	}

	return nil
}

// runMetricCollector runs a single metric collector in a loop
func (c *Collector) runMetricCollector(metric *metricCollector) {
	ticker := time.NewTicker(metric.interval)
	defer ticker.Stop()

	logger := c.logger.With("metric", metric.subject, "interval", metric.interval)
	logger.Debug("starting metric collector")

	c.collectAndPublish(metric, logger)

	for {
		select {
		case <-c.ctx.Done():
			logger.Debug("metric collector stopped")
			return
		case <-ticker.C:
			c.collectAndPublish(metric, logger)
		}
	}
}

// collectAndPublish collects metrics and publishes them to NATS
func (c *Collector) collectAndPublish(metric *metricCollector, logger *slog.Logger) {
	// Create timeout context for collection
	collectCtx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()

	flag := true
	defer func() {
		metric.success.Store(flag)
	}()

	// Collect metrics
	data, err := metric.collectFunc(collectCtx)
	if err != nil {
		flag = false
		logger.Error("failed to collect metrics", "error", err)
		return
	}

	// Marshal to JSON
	payload, err := json.Marshal(data)
	if err != nil {
		flag = false
		logger.Error("failed to marshal metrics", "error", err)
		return
	}

	// Publish to NATS
	subject := c.subjectPrefix + metric.subject
	if err := c.reporter.Publish(c.ctx, subject, payload); err != nil {
		flag = false
		logger.Error("failed to publish metrics", "subject", subject, "error", err)
		return
	}

	logger.Debug("metrics published", "subject", subject, "size", len(payload))
}
