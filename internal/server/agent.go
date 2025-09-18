// Package server provides the agent server implementation.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/telepair/watchdog/internal/agent"
	"github.com/telepair/watchdog/internal/config"
	"github.com/telepair/watchdog/pkg/health"
	"github.com/telepair/watchdog/pkg/logger"
	"github.com/telepair/watchdog/pkg/natsx/client"
	"github.com/telepair/watchdog/pkg/shutdown"
)

// AgentServer represents a dedicated agent server
type AgentServer struct {
	agent         *agent.Agent
	config        *config.Config
	natsClient    *client.Client
	healthManager *health.Server
	shutdownMgr   *shutdown.Manager
	logger        *slog.Logger
}

// NewAgent creates a new agent server with the given configuration
func NewAgent(cfg *config.Config) (*AgentServer, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Setup logger
	if err := logger.SetDefault(cfg.Logger); err != nil {
		return nil, fmt.Errorf("failed to setup logger: %w", err)
	}

	serverLogger := logger.ComponentLogger("wd.agent")

	// Create NATS client
	natsClient, err := client.NewClient(&cfg.NATS)
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS client: %w", err)
	}

	// Create health manager
	healthManager, err := health.NewServer(cfg.Health)
	if err != nil {
		return nil, fmt.Errorf("failed to create health manager: %w", err)
	}

	// Create shutdown manager
	shutdownMgr := shutdown.NewManager().
		WithTimeout(time.Duration(cfg.ShutdownTimeoutSec) * time.Second).
		WithLogger(logger.ComponentLogger("shutdown"))

	// Create agent instance
	agentInstance, err := agent.NewAgent(&cfg.Agent,
		&cfg.Storage.AgentBucket,
		&cfg.Storage.AgentStream,
		natsClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	agentServer := &AgentServer{
		agent:         agentInstance,
		config:        cfg,
		natsClient:    natsClient,
		healthManager: healthManager,
		logger:        serverLogger,
		shutdownMgr:   shutdownMgr,
	}

	// Register shutdown handlers
	agentServer.registerShutdownHandlers()

	return agentServer, nil
}

// Start starts the agent server
func (as *AgentServer) Start() error {
	as.logger.Info("starting agent server")

	// Start health server in background
	go func() {
		if err := as.healthManager.ListenAndServe(context.Background()); err != nil && err != context.Canceled {
			as.logger.Error("health server stopped unexpectedly", "error", err)
		}
	}()

	// Register health checks
	if err := as.registerHealthChecks(); err != nil {
		return fmt.Errorf("failed to register health checks: %w", err)
	}

	// Start agent
	if err := as.agent.Start(); err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	// Set ready state to true after all components are started
	as.healthManager.SetReady(true)

	as.logger.Info("agent server started successfully",
		"nats_connected", as.natsClient.IsConnected(),
		"health_addr", as.config.Health.Addr,
	)

	return nil
}

// Stop stops the agent server gracefully
func (as *AgentServer) Stop() error {
	return as.shutdownMgr.Shutdown()
}

// Wait blocks until a shutdown signal is received
func (as *AgentServer) Wait() error {
	as.logger.Info("agent server ready, waiting for shutdown signal...")
	return as.shutdownMgr.Wait()
}

// registerShutdownHandlers registers shutdown handlers for agent server components
func (as *AgentServer) registerShutdownHandlers() {
	// 0. Set ready state to false immediately when shutdown starts
	as.shutdownMgr.RegisterFunc(func(ctx context.Context) error {
		as.logger.Info("setting ready state to false for graceful shutdown...")
		as.healthManager.SetReady(false)
		return nil
	})

	// Stop agent if running
	if as.agent != nil {
		as.shutdownMgr.RegisterFunc(func(ctx context.Context) error {
			as.logger.Info("stopping agent...")
			return as.agent.Stop()
		})
	}

	as.shutdownMgr.RegisterFunc(func(ctx context.Context) error {
		as.logger.Info("stopping health manager...")
		return as.healthManager.Shutdown(ctx)
	})

	as.shutdownMgr.RegisterFunc(func(ctx context.Context) error {
		as.logger.Info("stopping NATS client...")
		return as.natsClient.Close()
	})
}

// registerHealthChecks registers health checks for agent server components
func (as *AgentServer) registerHealthChecks() error {
	// Register NATS connection health check
	if err := as.healthManager.RegisterChecker("nats-connection", 30*time.Second, func(ctx context.Context) error {
		if err := as.natsClient.HealthCheck(); err != nil {
			return fmt.Errorf("NATS health check failed: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to register NATS health check: %w", err)
	}

	return nil
}
