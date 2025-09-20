// Package server provides the main watchdog server implementation.
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
	"github.com/telepair/watchdog/pkg/natsx/embed"
	"github.com/telepair/watchdog/pkg/shutdown"
)

const (
	defaultShutdownTimeout = 10 * time.Second
	healthCheckInterval    = 30 * time.Second
)

// Server represents the unified server that can run as agent or main server
type Server struct {
	config        *config.Config
	agent         *agent.Agent
	embeddedNATS  *embed.EmbeddedServer
	natsClient    *client.Client
	healthManager *health.Server
	shutdownMgr   *shutdown.Manager
	logger        *slog.Logger
}

// NewServer creates a new main server with the given configuration
func NewServer(cfg *config.Config) (*Server, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	if err := cfg.Parse(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Setup logger
	if err := logger.SetDefault(cfg.Logger); err != nil {
		return nil, fmt.Errorf("failed to setup logger: %w", err)
	}

	srv := &Server{
		config: cfg,
		logger: logger.ComponentLogger("wd.server"),
	}

	var err error
	// Create embedded NATS server if needed
	if cfg.Server.EnableEmbedNATS {
		srv.embeddedNATS, err = embed.NewEmbeddedServer(cfg.Server.EmbedNATS)
		if err != nil {
			return nil, fmt.Errorf("failed to create embedded NATS server: %w", err)
		}
		if err := srv.embeddedNATS.Start(); err != nil {
			return nil, fmt.Errorf("failed to start embedded NATS server: %w", err)
		}
		srv.logger.Info("embedded NATS server started", "url", srv.embeddedNATS.ClientURL())
	}

	// Create NATS client
	srv.natsClient, err = client.NewClient(&cfg.NATS)
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS client: %w", err)
	}

	// Initialize NATS infrastructure (KV buckets and streams)
	if err := srv.initializeNATSInfrastructure(); err != nil {
		return nil, fmt.Errorf("failed to initialize NATS infrastructure: %w", err)
	}

	// Create health manager
	srv.healthManager, err = health.NewServer(cfg.HealthAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create health manager: %w", err)
	}

	// Create shutdown manager
	srv.shutdownMgr = shutdown.NewManager().
		WithTimeout(defaultShutdownTimeout).
		WithLogger(logger.ComponentLogger("shutdown"))

		// Create agent if embedded agent is enabled
	srv.agent, err = agent.NewAgent(&cfg.Agent, &cfg.Collector, srv.natsClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Register shutdown handlers
	srv.registerShutdownHandlers()

	return srv, nil
}

// Start starts all server components
func (s *Server) Start() error {
	s.logger.Info("starting main server",
		"embedded_nats", s.config.Server.EnableEmbedNATS,
	)

	// Start health server in background with recovery mechanism
	healthErrorChan := make(chan error, 1)
	go func() {
		defer close(healthErrorChan)
		if err := s.healthManager.ListenAndServe(); err != nil && err != context.Canceled {
			s.logger.Error("health server stopped unexpectedly", "error", err)
			select {
			case healthErrorChan <- err:
			default:
			}
		}
	}()

	// Monitor health server errors
	go func() {
		for err := range healthErrorChan {
			s.logger.Warn("health server error detected", "error", err)
			// In production, consider implementing restart logic or circuit breaker
			// For now, we just log the error for monitoring purposes
		}
	}()

	// Register health checks
	if err := s.registerHealthChecks(); err != nil {
		return fmt.Errorf("failed to register health checks: %w", err)
	}

	// Start agent if available
	if s.agent != nil {
		if err := s.agent.Start(); err != nil {
			return fmt.Errorf("failed to start agent: %w", err)
		}
	}

	// Set ready state to true after all components are started
	s.healthManager.SetReady(true)

	s.logger.Info("watchdog started successfully",
		"nats_connected", s.natsClient.IsConnected(),
		"health_addr", s.config.HealthAddr,
		"agent_running", s.agent != nil,
		"embedded_nats_running", s.embeddedNATS != nil && s.embeddedNATS.IsRunning(),
	)

	return nil
}

// Stop stops all server components gracefully.
func (s *Server) Stop() error {
	return s.shutdownMgr.Shutdown()
}

// Wait blocks until a shutdown signal is received
func (s *Server) Wait() error {
	s.logger.Info("watchdog ready, waiting for shutdown signal")
	return s.shutdownMgr.Wait()
}

// initializeNATSInfrastructure initializes KV buckets and streams required by agents
func (s *Server) initializeNATSInfrastructure() error {
	if _, err := s.natsClient.EnsureBucket(context.Background(), s.config.Collector.AgentBucket); err != nil {
		s.logger.Error("failed to ensure agent bucket", "error", err, "bucket", s.config.Collector.AgentBucket.Bucket)
		return fmt.Errorf("failed to ensure agent bucket: %w", err)
	}

	if _, err := s.natsClient.EnsureStream(context.Background(), s.config.Collector.AgentStream); err != nil {
		s.logger.Error("failed to ensure agent stream", "error", err, "stream", s.config.Collector.AgentStream.Name)
		return fmt.Errorf("failed to ensure agent stream: %w", err)
	}

	s.logger.Info("NATS infrastructure initialized successfully")
	return nil
}

// Shutdown order is important: stop dependent services first, then infrastructure components.
func (s *Server) registerShutdownHandlers() {
	// 0. Set ready state to false immediately when shutdown starts
	s.shutdownMgr.RegisterFunc(func(ctx context.Context) error {
		s.logger.Info("setting ready state to false for graceful shutdown...")
		s.healthManager.SetReady(false)
		return nil
	})

	// 1. Stop agent first (depends on NATS)
	if s.agent != nil {
		s.shutdownMgr.RegisterFunc(func(ctx context.Context) error {
			s.logger.Info("stopping agent...")
			return s.agent.Stop()
		})
	}

	// 2. Stop health manager
	s.shutdownMgr.RegisterFunc(func(ctx context.Context) error {
		s.logger.Info("stopping health manager...")
		return s.healthManager.Shutdown(ctx)
	})

	// 3. Stop NATS client
	s.shutdownMgr.RegisterFunc(func(ctx context.Context) error {
		s.logger.Info("stopping NATS client...")
		return s.natsClient.Close()
	})

	// 4. Stop embedded NATS server last (infrastructure)
	if s.embeddedNATS != nil {
		s.shutdownMgr.RegisterFunc(func(ctx context.Context) error {
			s.logger.Info("stopping embedded NATS server...")
			return s.embeddedNATS.Stop()
		})
	}
}

// registerHealthChecks registers health checks for server components.
func (s *Server) registerHealthChecks() error {
	// Register NATS connection health check
	if err := s.healthManager.RegisterChecker("nats-connection", healthCheckInterval, func() error {
		if err := s.natsClient.HealthCheck(); err != nil {
			return fmt.Errorf("NATS health check failed: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to register NATS health check: %w", err)
	}

	// Register embedded NATS server health check
	if s.embeddedNATS != nil {
		if err := s.healthManager.RegisterChecker("embedded-nats", healthCheckInterval, func() error {
			if err := s.embeddedNATS.HealthCheck(); err != nil {
				return fmt.Errorf("embedded NATS health check failed: %w", err)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed to register embedded NATS health check: %w", err)
		}
	}

	return nil
}
