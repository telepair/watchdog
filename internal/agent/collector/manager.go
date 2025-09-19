package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/telepair/watchdog/internal/config"
	"github.com/telepair/watchdog/pkg/logger"
	"github.com/telepair/watchdog/pkg/natsx/client"
)

// Manager manages metrics collection and publishing
type Manager struct {
	agentID   string
	cfg       *config.CollectorConfig
	streamCfg config.AgentStreamConfig
	nats      *client.Client
	ctx       context.Context
	cancel    context.CancelFunc
	running   atomic.Bool
	logger    *slog.Logger
}

// NewManager creates a new collector manager
func NewManager(agentID string,
	natsClient *client.Client,
	cfg *config.CollectorConfig,
	streamCfg *config.AgentStreamConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		agentID:   agentID,
		cfg:       cfg,
		streamCfg: *streamCfg,
		nats:      natsClient,
		ctx:       ctx,
		cancel:    cancel,
		running:   atomic.Bool{},
		logger:    logger.ComponentLogger("agent.collector"),
	}
}

// Start starts the metrics collection
func (m *Manager) Start() error {
	if m.running.Load() {
		return fmt.Errorf("collector already running")
	}

	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.running.Store(true)

	m.logger.Info("starting metrics collector",
		"agent_id", m.agentID,
		"metric_interval", m.cfg.ReportIntervalSec,
	)

	// Start metrics collection goroutine
	go m.collectMetrics()

	return nil
}

// Stop stops the metrics collection
func (m *Manager) Stop() error {
	if !m.running.Load() {
		return nil
	}

	m.logger.Info("stopping metrics collector")
	m.running.Store(false)
	m.cancel()

	return nil
}

// IsRunning returns whether the collector is running
func (m *Manager) IsRunning() bool {
	return m.running.Load()
}

// collectMetrics runs the metrics collection loop
func (m *Manager) collectMetrics() {
	ticker := time.NewTicker(time.Duration(m.cfg.ReportIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.collectCpuInfo()
			m.collectMemoryInfo()
			m.collectDiskInfo()
			m.collectNetworkInfo()
			m.collectLoadInfo()
			m.collectUptimeInfo()
		}
	}
}

func (m *Manager) collectCpuInfo() {
	if !m.cfg.CollectCPU {
		return
	}

	metrics, err := CollectCPU(m.ctx)
	if err != nil {
		m.logger.Error("failed to collect CPU metrics", "error", err)
		return
	}
	data, err := json.Marshal(metrics)
	if err != nil {
		m.logger.Error("failed to marshal CPU metrics", "error", err)
		return
	}
	subject := m.streamCfg.SysCPUSubject(m.agentID)
	if err := m.nats.Publish(subject, data); err != nil {
		m.logger.Error("failed to publish CPU metrics", "error", err)
	}
}

func (m *Manager) collectMemoryInfo() {
	if !m.cfg.CollectMemory {
		return
	}

	metrics, err := CollectMemory(m.ctx)
	if err != nil {
		m.logger.Error("failed to collect memory metrics", "error", err)
		return
	}
	data, err := json.Marshal(metrics)
	if err != nil {
		m.logger.Error("failed to marshal memory metrics", "error", err)
		return
	}
	subject := m.streamCfg.SysMemorySubject(m.agentID)
	if err := m.nats.Publish(subject, data); err != nil {
		m.logger.Error("failed to publish memory metrics", "error", err)
	}
}

func (m *Manager) collectDiskInfo() {
	if !m.cfg.CollectDisk {
		return
	}
	metrics, err := CollectDisk(m.ctx)
	if err != nil {
		m.logger.Error("failed to collect disk metrics", "error", err)
		return
	}
	data, err := json.Marshal(metrics)
	if err != nil {
		m.logger.Error("failed to marshal disk metrics", "error", err)
		return
	}
	subject := m.streamCfg.SysDiskSubject(m.agentID)
	if err := m.nats.Publish(subject, data); err != nil {
		m.logger.Error("failed to publish disk metrics", "error", err)
	}
}

func (m *Manager) collectNetworkInfo() {
	if !m.cfg.CollectNetwork {
		return
	}

	metrics, err := CollectNetwork(m.ctx)
	if err != nil {
		m.logger.Error("failed to collect network metrics", "error", err)
		return
	}
	data, err := json.Marshal(metrics)
	if err != nil {
		m.logger.Error("failed to marshal network metrics", "error", err)
		return
	}
	subject := m.streamCfg.SysNetworkSubject(m.agentID)
	if err := m.nats.Publish(subject, data); err != nil {
		m.logger.Error("failed to publish network metrics", "error", err)
	}
}

func (m *Manager) collectLoadInfo() {
	if !m.cfg.CollectLoad {
		return
	}
	metrics, err := CollectLoad(m.ctx)
	if err != nil {
		m.logger.Error("failed to collect load metrics", "error", err)
		return
	}
	data, err := json.Marshal(metrics)
	if err != nil {
		m.logger.Error("failed to marshal load metrics", "error", err)
		return
	}
	subject := m.streamCfg.SysLoadSubject(m.agentID)
	if err := m.nats.Publish(subject, data); err != nil {
		m.logger.Error("failed to publish load metrics", "error", err)
	}
}

func (m *Manager) collectUptimeInfo() {
	if !m.cfg.CollectUptime {
		return
	}

	metrics, err := CollectUptime(m.ctx)
	if err != nil {
		m.logger.Error("failed to collect uptime metrics", "error", err)
		return
	}
	data, err := json.Marshal(metrics)
	if err != nil {
		m.logger.Error("failed to marshal uptime metrics", "error", err)
		return
	}
	subject := m.streamCfg.SysUptimeSubject(m.agentID)
	if err := m.nats.Publish(subject, data); err != nil {
		m.logger.Error("failed to publish uptime metrics", "error", err)
	}
}
