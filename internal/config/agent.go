package config

import (
	"errors"
)

var (
	defaultAgentID              = "watchdog-agent"
	defaultMetricReportInterval = 10
	defaultInfoReportInterval   = 600
	defaultHeartbeatInterval    = 5
)

// AgentConfig holds agent-specific configuration
type AgentConfig struct {
	ID                 string           `yaml:"id" json:"id"`
	Collector          *CollectorConfig `yaml:"collector" json:"collector"`
	InfoReportInterval int              `yaml:"info_report_interval" json:"info_report_interval"`
	HeartbeatInterval  int              `yaml:"heartbeat_interval" json:"heartbeat_interval"`
}

func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		ID:                 defaultAgentID,
		Collector:          DefaultCollectorConfig(),
		InfoReportInterval: defaultInfoReportInterval,
		HeartbeatInterval:  defaultHeartbeatInterval,
	}
}

func (c *AgentConfig) Validate() error {
	if c.ID == "" {
		return errors.New("invalid agent ID")
	}
	if c.Collector == nil {
		return errors.New("invalid collector config")
	}
	if err := c.Collector.Validate(); err != nil {
		return err
	}
	if c.InfoReportInterval <= 0 {
		return errors.New("invalid info report interval")
	}
	if c.HeartbeatInterval <= 0 {
		return errors.New("invalid heartbeat interval")
	}
	return nil
}

func (c *AgentConfig) SetDefaults() {
	if c.ID == "" {
		c.ID = defaultAgentID
	}
	if c.Collector == nil {
		c.Collector = DefaultCollectorConfig()
	}
	if c.InfoReportInterval <= 0 {
		c.InfoReportInterval = defaultInfoReportInterval
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = defaultHeartbeatInterval
	}
	c.Collector.SetDefaults()
}

type CollectorConfig struct {
	ReportIntervalSec int  `yaml:"report_interval_sec" json:"report_interval_sec"`
	CollectCPU        bool `yaml:"collect_cpu" json:"collect_cpu"`
	CollectMemory     bool `yaml:"collect_memory" json:"collect_memory"`
	CollectDisk       bool `yaml:"collect_disk" json:"collect_disk"`
	CollectNetwork    bool `yaml:"collect_network" json:"collect_network"`
	CollectLoad       bool `yaml:"collect_load" json:"collect_load"`
	CollectUptime     bool `yaml:"collect_uptime" json:"collect_uptime"`
}

func DefaultCollectorConfig() *CollectorConfig {
	return &CollectorConfig{
		ReportIntervalSec: defaultMetricReportInterval,
		CollectCPU:        true,
		CollectMemory:     true,
		CollectDisk:       true,
		CollectNetwork:    true,
		CollectLoad:       true,
		CollectUptime:     true,
	}
}

func (c *CollectorConfig) Validate() error {
	if c.ReportIntervalSec <= 0 {
		return errors.New("invalid report interval")
	}
	return nil
}

func (c *CollectorConfig) SetDefaults() {
	if c.ReportIntervalSec <= 0 {
		c.ReportIntervalSec = defaultMetricReportInterval
	}
}
