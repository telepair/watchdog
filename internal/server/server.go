// Package server provides the main watchdog server implementation.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/telepair/watchdog/internal/agent"
	"github.com/telepair/watchdog/internal/config"
	"github.com/telepair/watchdog/pkg/health"
	"github.com/telepair/watchdog/pkg/logger"
	"github.com/telepair/watchdog/pkg/natsx/client"
	"github.com/telepair/watchdog/pkg/natsx/embed"
	"github.com/telepair/watchdog/pkg/shutdown"
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

	if err := cfg.Validate(); err != nil {
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

	// Create health manager
	srv.healthManager, err = health.NewServer(cfg.Health)
	if err != nil {
		return nil, fmt.Errorf("failed to create health manager: %w", err)
	}

	// Create shutdown manager
	srv.shutdownMgr = shutdown.NewManager().
		WithTimeout(10 * time.Second).
		WithLogger(logger.ComponentLogger("shutdown"))

		// Create agent if embedded agent is enabled
	srv.agent, err = agent.NewAgent(&cfg.Agent,
		&cfg.Storage.AgentBucket,
		&cfg.Storage.AgentStream,
		srv.natsClient)
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

	// Start health server in background
	go func() {
		if err := s.healthManager.ListenAndServe(context.Background()); err != nil && err != context.Canceled {
			s.logger.Error("health server stopped unexpectedly", "error", err)
		}
	}()

	// Initialize NATS infrastructure (KV buckets and streams)
	if err := s.initializeNATSInfrastructure(); err != nil {
		return fmt.Errorf("failed to initialize NATS infrastructure: %w", err)
	}

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
		"health_addr", s.config.Health.Addr,
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	js := s.natsClient.JetStream()
	if js == nil {
		// Check if NATS is connected first for better error message
		if !s.natsClient.IsConnected() {
			return fmt.Errorf("JetStream not available: NATS client not connected")
		}
		return fmt.Errorf("JetStream not available: feature may not be enabled on NATS server")
	}

	// Initialize Agent bucket for agent data
	if err := s.initializeAgentBucket(ctx, js); err != nil {
		return fmt.Errorf("failed to initialize KV bucket: %w", err)
	}

	// Initialize JetStream for agent communications
	if err := s.initializeAgentStream(ctx, js); err != nil {
		return fmt.Errorf("failed to initialize agent stream: %w", err)
	}

	s.logger.Info("NATS infrastructure initialized successfully")
	return nil
}

// initializeAgentBucket creates or ensures the agent KV bucket exists
func (s *Server) initializeAgentBucket(ctx context.Context, js jetstream.JetStream) error {
	bucketCfg := s.config.Storage.AgentBucket

	var storage jetstream.StorageType
	switch bucketCfg.Storage {
	case "memory":
		storage = jetstream.MemoryStorage
	default:
		storage = jetstream.FileStorage
	}

	bucketConfig := jetstream.KeyValueConfig{
		Bucket:      bucketCfg.Name,
		History:     bucketCfg.History,
		TTL:         bucketCfg.TTL,
		Storage:     storage,
		Replicas:    bucketCfg.Replicas,
		Compression: bucketCfg.Compression,
	}

	_, err := js.CreateKeyValue(ctx, bucketConfig)
	if err != nil {
		// Try to get existing bucket
		_, err = js.KeyValue(ctx, bucketCfg.Name)
		if err != nil {
			s.logger.Error("failed to create or get KV bucket", "bucket", bucketCfg.BucketName(), "error", err)
			return fmt.Errorf("failed to create or get KV bucket %s: %w", bucketCfg.BucketName(), err)
		}
	}

	s.logger.Info("KV bucket initialized", "bucket", bucketCfg.BucketName())
	return nil
}

// initializeAgentStream creates or ensures the agent stream exists
func (s *Server) initializeAgentStream(ctx context.Context, js jetstream.JetStream) error {
	streamCfg := s.config.Storage.AgentStream

	var retention jetstream.RetentionPolicy
	switch streamCfg.Retention {
	case "interest":
		retention = jetstream.InterestPolicy
	case "workqueue":
		retention = jetstream.WorkQueuePolicy
	default:
		retention = jetstream.LimitsPolicy
	}

	var storage jetstream.StorageType
	switch streamCfg.Storage {
	case "memory":
		storage = jetstream.MemoryStorage
	default:
		storage = jetstream.FileStorage
	}

	streamConfig := jetstream.StreamConfig{
		Name:        streamCfg.Name,
		Description: streamCfg.Description,
		Subjects:    []string{streamCfg.SubjectPattern},
		Retention:   retention,
		MaxAge:      streamCfg.MaxAge,
		MaxBytes:    streamCfg.MaxBytes,
		MaxMsgs:     streamCfg.MaxMsgs,
		Storage:     storage,
		Replicas:    streamCfg.Replicas,
		NoAck:       streamCfg.NoAck,
		Duplicates:  streamCfg.Duplicates,
	}

	_, err := js.CreateStream(ctx, streamConfig)
	if err != nil {
		// Try to get existing stream
		_, err = js.Stream(ctx, streamCfg.Name)
		if err != nil {
			s.logger.Error("failed to create or get stream", "stream", streamCfg.Name, "error", err)
			return fmt.Errorf("failed to create or get stream %s: %w", streamCfg.Name, err)
		}
	}

	s.logger.Info("agent stream initialized", "stream", streamCfg.Name)
	return nil
}

// registerShutdownHandlers registers shutdown handlers for all components.
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
	if err := s.healthManager.RegisterChecker("nats-connection", 30*time.Second, func(ctx context.Context) error {
		if err := s.natsClient.HealthCheck(); err != nil {
			return fmt.Errorf("NATS health check failed: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to register NATS health check: %w", err)
	}

	// Register embedded NATS server health check
	if s.embeddedNATS != nil {
		if err := s.healthManager.RegisterChecker("embedded-nats", 30*time.Second, func(ctx context.Context) error {
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
