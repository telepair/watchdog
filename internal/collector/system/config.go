package system

import "time"

const (
	defaultCPUSubjectSuffix     = "cpu"
	defaultMemorySubjectSuffix  = "mem"
	defaultDiskSubjectSuffix    = "disk"
	defaultNetworkSubjectSuffix = "net"
	defaultLoadSubjectSuffix    = "load"
	defaultUptimeSubjectSuffix  = "uptime"

	defaultReportIntervalSec = 10
)

// CollectorMetric represents configuration for a single metric type
type CollectorMetric struct {
	Enabled         bool   `yaml:"enabled" json:"enabled"`
	SubjectSuffix   string `yaml:"subject_suffix" json:"subject_suffix"`
	IntervalSeconds int    `yaml:"interval_seconds" json:"interval_seconds"`
}

// newDefaultMetric creates a metric with default configuration
func newDefaultMetric(suffix string) *CollectorMetric {
	return &CollectorMetric{
		Enabled:         true,
		SubjectSuffix:   suffix,
		IntervalSeconds: defaultReportIntervalSec,
	}
}

// GetInterval returns the effective collection interval for a metric,
// falling back to global interval if not specified
func (m *CollectorMetric) GetInterval() time.Duration {
	if m == nil || m.IntervalSeconds <= 0 {
		return time.Duration(defaultReportIntervalSec) * time.Second
	}
	return time.Duration(m.IntervalSeconds) * time.Second
}

// IsEnabled returns whether the metric collection is enabled
func (m *CollectorMetric) IsEnabled() bool {
	return m != nil && m.Enabled
}

// Config holds configuration for all system metrics collection
type Config struct {
	GlobalInterval int              `yaml:"global_interval" json:"global_interval"`
	CPU            *CollectorMetric `yaml:"cpu" json:"cpu"`
	Memory         *CollectorMetric `yaml:"memory" json:"memory"`
	Disk           *CollectorMetric `yaml:"disk" json:"disk"`
	Network        *CollectorMetric `yaml:"network" json:"network"`
	Load           *CollectorMetric `yaml:"load" json:"load"`
	Uptime         *CollectorMetric `yaml:"uptime" json:"uptime"`
}

// DefaultSystemCollectorConfig returns a configuration with all metrics enabled using default values
func DefaultConfig() Config {
	return Config{
		GlobalInterval: defaultReportIntervalSec,
		CPU:            newDefaultMetric(defaultCPUSubjectSuffix),
		Memory:         newDefaultMetric(defaultMemorySubjectSuffix),
		Disk:           newDefaultMetric(defaultDiskSubjectSuffix),
		Network:        newDefaultMetric(defaultNetworkSubjectSuffix),
		Load:           newDefaultMetric(defaultLoadSubjectSuffix),
		Uptime:         newDefaultMetric(defaultUptimeSubjectSuffix),
	}
}

// parseMetric validates and applies defaults to a single metric configuration
func (c *Config) parseMetric(metric *CollectorMetric, defaultSuffix string) {
	if metric == nil {
		return
	}
	if metric.SubjectSuffix == "" {
		metric.SubjectSuffix = defaultSuffix
	}
	if metric.IntervalSeconds <= 0 {
		if c.GlobalInterval > 0 {
			metric.IntervalSeconds = c.GlobalInterval
		} else {
			metric.IntervalSeconds = defaultReportIntervalSec
		}
	}
}

// Parse validates and applies defaults to the configuration
func (c *Config) Parse() error {
	// Apply global interval default
	if c.GlobalInterval <= 0 {
		c.GlobalInterval = defaultReportIntervalSec
	}

	// Initialize nil metrics with defaults
	if c.CPU == nil {
		c.CPU = newDefaultMetric(defaultCPUSubjectSuffix)
	}
	if c.Memory == nil {
		c.Memory = newDefaultMetric(defaultMemorySubjectSuffix)
	}
	if c.Disk == nil {
		c.Disk = newDefaultMetric(defaultDiskSubjectSuffix)
	}
	if c.Network == nil {
		c.Network = newDefaultMetric(defaultNetworkSubjectSuffix)
	}
	if c.Load == nil {
		c.Load = newDefaultMetric(defaultLoadSubjectSuffix)
	}
	if c.Uptime == nil {
		c.Uptime = newDefaultMetric(defaultUptimeSubjectSuffix)
	}

	// Parse individual metrics
	c.parseMetric(c.CPU, defaultCPUSubjectSuffix)
	c.parseMetric(c.Memory, defaultMemorySubjectSuffix)
	c.parseMetric(c.Disk, defaultDiskSubjectSuffix)
	c.parseMetric(c.Network, defaultNetworkSubjectSuffix)
	c.parseMetric(c.Load, defaultLoadSubjectSuffix)
	c.parseMetric(c.Uptime, defaultUptimeSubjectSuffix)

	return nil
}
