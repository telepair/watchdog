package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/telepair/watchdog/internal/agent/collector"
	"github.com/telepair/watchdog/internal/agent/executor"
	"github.com/telepair/watchdog/internal/config"
	"github.com/telepair/watchdog/pkg/logger"
	"github.com/telepair/watchdog/pkg/natsx/client"
)

// Agent represents the main agent that coordinates collector and executor
type Agent struct {
	config     *config.AgentConfig
	bucketCfg  *config.AgentBucketConfig
	streamCfg  *config.AgentStreamConfig
	natsClient *client.Client
	collector  *collector.Manager
	executor   executor.Executor
	logger     *slog.Logger

	info      *AgentInfo
	status    *AgentStatus
	startedAt time.Time

	// Timer control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewAgent creates a new agent instance
func NewAgent(agentConfig *config.AgentConfig,
	bucketCfg *config.AgentBucketConfig,
	streamCfg *config.AgentStreamConfig,
	natsClient *client.Client) (*Agent, error) {
	if agentConfig == nil {
		return nil, fmt.Errorf("agent config is required")
	}

	if natsClient == nil {
		return nil, fmt.Errorf("NATS client is required")
	}

	// Create collectors
	collectorManager := collector.NewManager(agentConfig.ID, natsClient, agentConfig.Collector, streamCfg)

	// Create executor
	natsListener := executor.NewNATSListener(natsClient, streamCfg)
	natsResultPublisher := executor.NewNATSResultPublisher(natsClient, streamCfg)

	executorManager := executor.NewManager(agentConfig.ID, natsListener, natsResultPublisher)

	ctx, cancel := context.WithCancel(context.Background())
	agent := &Agent{
		config:     agentConfig,
		bucketCfg:  bucketCfg,
		streamCfg:  streamCfg,
		natsClient: natsClient,
		collector:  collectorManager,
		executor:   executorManager,
		logger:     logger.ComponentLogger("agent"),
		startedAt:  time.Now(),
		ctx:        ctx,
		cancel:     cancel,
	}

	// Initialize agent info and status
	agent.initializeInfo()
	agent.initializeStatus()

	return agent, nil
}

// Start starts the agent and all its components
func (a *Agent) Start() error {
	a.logger.Info("starting agent", "agent_id", a.config.ID)

	// Update status
	a.status.Status = "running"
	a.status.UpdatedAt = time.Now()

	// Publish agent info and initial status
	if err := a.updateInfo(); err != nil {
		a.logger.Error("failed to publish agent info", "error", err)
	}

	if err := a.updateStatus(); err != nil {
		a.logger.Error("failed to publish agent status", "error", err)
	}

	// Publish startup event
	if err := a.publishEvent(config.EventTypeStartup); err != nil {
		a.logger.Error("failed to publish startup event", "error", err)
	}

	// Start collector
	if err := a.collector.Start(); err != nil {
		return fmt.Errorf("failed to start collector: %w", err)
	}
	a.status.CollectorRunning = a.collector.IsRunning()

	// Start executor
	if err := a.executor.Start(); err != nil {
		return fmt.Errorf("failed to start executor: %w", err)
	}
	a.status.ExecutorRunning = a.executor.IsRunning()

	// Update status after starting components
	a.status.UpdatedAt = time.Now()
	if err := a.updateStatus(); err != nil {
		a.logger.Error("failed to publish updated status", "error", err)
	}

	a.startReportTimers()

	a.logger.Info("agent started successfully",
		"agent_id", a.config.ID,
		"collector_running", a.status.CollectorRunning,
		"executor_running", a.status.ExecutorRunning,
	)

	return nil
}

// Stop stops the agent and all its components
func (a *Agent) Stop() error {
	a.logger.Info("stopping agent", "agent_id", a.config.ID)

	// Stop periodic reporting timers
	if a.cancel != nil {
		a.cancel()
		a.wg.Wait() // Wait for all timer goroutines to finish
	}

	// Update status
	a.status.Status = "stopping"
	a.status.UpdatedAt = time.Now()

	if err := a.updateStatus(); err != nil {
		a.logger.Error("failed to publish stopping status", "error", err)
	}

	// Publish shutdown event
	if err := a.publishEvent(config.EventTypeShutdown); err != nil {
		a.logger.Error("failed to publish shutdown event", "error", err)
	}

	// Stop components
	if err := a.collector.Stop(); err != nil {
		a.logger.Error("failed to stop collector", "error", err)
	}

	if err := a.executor.Stop(); err != nil {
		a.logger.Error("failed to stop executor", "error", err)
	}

	// Update final status
	a.status.Status = "stopped"
	a.status.CollectorRunning = false
	a.status.ExecutorRunning = false
	a.status.UpdatedAt = time.Now()

	if err := a.updateStatus(); err != nil {
		a.logger.Error("failed to publish final status", "error", err)
	}

	a.logger.Info("agent stopped successfully")
	return nil
}
