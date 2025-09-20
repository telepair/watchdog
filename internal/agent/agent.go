package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/telepair/watchdog/internal/collector"
	"github.com/telepair/watchdog/internal/reporter"
	"github.com/telepair/watchdog/pkg/natsx/client"
)

// Agent represents the main agent that coordinates collector and executor
type Agent struct {
	config       *Config
	collectorCfg *collector.Config
	natsClient   *client.Client

	collector *collector.Manager
	bucket    *reporter.Bucket

	running   atomic.Bool
	startedAt time.Time

	// Timer control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	logger *slog.Logger
}

// NewAgent creates a new agent instance
func NewAgent(cfg *Config, collectorCfg *collector.Config, natsClient *client.Client) (*Agent, error) {
	if cfg == nil {
		return nil, fmt.Errorf("agent config is required")
	}

	if natsClient == nil {
		return nil, fmt.Errorf("NATS client is required")
	}

	bucket, err := reporter.GetBucket(collectorCfg.AgentBucket.Bucket, natsClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create reporter: %w", err)
	}

	stream, err := reporter.GetStream(collectorCfg.AgentStream.Name, natsClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create reporter: %w", err)
	}

	// Create collectors
	collectorManager, err := collector.NewManager(cfg.ID, collectorCfg, stream)
	if err != nil {
		return nil, fmt.Errorf("failed to create collector manager: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	agent := &Agent{
		config:       cfg,
		collectorCfg: collectorCfg,
		natsClient:   natsClient,
		collector:    collectorManager,
		bucket:       bucket,
		startedAt:    time.Now(),
		ctx:          ctx,
		cancel:       cancel,
		logger:       slog.Default().With("component", "wd-agent", "agent_id", cfg.ID),
	}
	return agent, nil
}

// Start starts the agent and all its components
func (a *Agent) Start() error {
	a.logger.Info("starting agent")
	a.startedAt = time.Now()
	a.running.Store(true)

	// Start collector
	if err := a.collector.Start(); err != nil {
		return fmt.Errorf("failed to start collector: %w", err)
	}

	a.startReport()

	a.logger.Info("agent started successfully")
	return nil
}

// Stop stops the agent and all its components
func (a *Agent) Stop() error {
	a.logger.Info("stopping agent")
	a.running.Store(false)

	// Stop periodic reporting timers
	if a.cancel != nil {
		a.cancel()
		a.wg.Wait() // Wait for all timer goroutines to finish
	}

	// Stop components
	if err := a.collector.Stop(); err != nil {
		a.logger.Error("failed to stop collector", "error", err)
	}

	a.logger.Info("agent stopped successfully")
	return nil
}
