package embed

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

const (
	// DefaultStartTimeout is the timeout for starting the embedded NATS server.
	DefaultStartTimeout = 10 * time.Second
	// DefaultShutdownTimeout is the timeout for shutting down the embedded NATS server.
	DefaultShutdownTimeout = 10 * time.Second
	// DefaultPort is the default NATS server port.
	DefaultPort = 4222
	// DefaultMaxMemoryMB is the default maximum memory for JetStream (64MB).
	DefaultMaxMemoryMB = 64 * 1024 * 1024
	// DefaultMaxStorageGB is the default maximum storage for JetStream (1GB).
	DefaultMaxStorageGB = 1024 * 1024 * 1024
	// DefaultWriteDeadline is the default write deadline for connections.
	DefaultWriteDeadline = 2 * time.Second
	// MaxPortNumber is the maximum valid port number.
	MaxPortNumber = 65535
	// MaxInt63 is the maximum value for int64 to prevent overflow.
	MaxInt63 = 1<<63 - 1
)

// ServerConfig holds embedded NATS server configuration.
type ServerConfig struct {
	Host          string        `yaml:"host"           json:"host"`
	Port          int           `yaml:"port"           json:"port"`
	StorePath     string        `yaml:"store_path"     json:"store_path"`
	MaxMemory     int64         `yaml:"max_memory"     json:"max_memory"`
	MaxStorage    int64         `yaml:"max_storage"    json:"max_storage"`
	LogLevel      string        `yaml:"log_level"      json:"log_level"`
	WriteDeadline time.Duration `yaml:"write_deadline" json:"write_deadline"`
}

// Validate validates the server configuration.
func (sc *ServerConfig) Validate() error {
	if sc.Port == -1 {
		sc.Port = 0 // Let system choose random port
	} else if sc.Port <= 0 || sc.Port > MaxPortNumber {
		sc.Port = DefaultPort
	}

	if sc.Host == "" {
		sc.Host = "127.0.0.1"
	}

	if sc.MaxMemory <= 0 {
		sc.MaxMemory = DefaultMaxMemoryMB
	}
	if sc.MaxStorage <= 0 {
		sc.MaxStorage = DefaultMaxStorageGB
	}
	if sc.StorePath == "" {
		sc.StorePath = "./data/nats"
	}

	if sc.WriteDeadline <= 0 {
		sc.WriteDeadline = DefaultWriteDeadline
	}
	return nil
}

// DefaultServerConfig returns a default NATS server configuration.
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:          "127.0.0.1",
		Port:          DefaultPort,
		StorePath:     "./data/nats",
		MaxMemory:     DefaultMaxMemoryMB,
		MaxStorage:    DefaultMaxStorageGB,
		LogLevel:      "INFO",
		WriteDeadline: DefaultWriteDeadline,
	}
}

// EmbeddedServer wraps a NATS server for embedded use.
type EmbeddedServer struct {
	server  *server.Server
	config  *ServerConfig
	logger  *slog.Logger
	mu      sync.RWMutex
	stopped bool
}

// NewEmbeddedServer creates a new embedded NATS server.
func NewEmbeddedServer(config *ServerConfig) (*EmbeddedServer, error) {
	if config == nil {
		config = DefaultServerConfig()
	}

	opts := &server.Options{
		Host:          config.Host,
		Port:          config.Port,
		WriteDeadline: config.WriteDeadline,
		NoLog:         true, // may be overridden by LogLevel below
	}

	// Set log level
	switch config.LogLevel {
	case "DEBUG":
		opts.Debug = true
		opts.NoLog = false
	case "TRACE":
		opts.Trace = true
		opts.NoLog = false
	case "INFO":
		// enable default logging
		opts.NoLog = false
	}

	// JetStream configuration
	jsConfig := &server.JetStreamConfig{
		MaxMemory: config.MaxMemory,
		MaxStore:  config.MaxStorage,
	}

	if config.StorePath != "" {
		jsConfig.StoreDir = filepath.Clean(config.StorePath)
	}

	opts.JetStream = true
	opts.JetStreamMaxMemory = jsConfig.MaxMemory
	opts.JetStreamMaxStore = jsConfig.MaxStore
	opts.StoreDir = jsConfig.StoreDir

	logger := slog.Default().With("component", "nats-embedded")
	// Create server
	s, err := server.NewServer(opts)
	if err != nil {
		logger.Error("failed to create NATS server", "error", err)
		return nil, fmt.Errorf("failed to create NATS server: %w", err)
	}

	return &EmbeddedServer{
		server: s,
		config: config,
		logger: logger,
	}, nil
}

// Start starts the embedded NATS server.
func (s *EmbeddedServer) Start() error {
	s.logger.Info("starting NATS server")
	s.server.Start()

	// Wait for server to be ready
	if !s.server.ReadyForConnections(DefaultStartTimeout) {
		s.logger.Error("NATS server failed to start within timeout")
		return errors.New("NATS server failed to start within timeout")
	}

	s.logger.Info("NATS server started")
	return nil
}

// Stop stops the embedded NATS server.
func (s *EmbeddedServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return nil
	}

	s.logger.Info("stopping NATS server")
	if s.server != nil && s.server.Running() {
		// Attempt graceful shutdown first
		s.server.Shutdown()

		// Wait for shutdown with timeout
		done := make(chan struct{})
		go func() {
			defer func() {
				// Recover from any panic during shutdown
				if r := recover(); r != nil {
					s.logger.Warn("recovered from panic during NATS shutdown", "panic", r)
				}
				close(done)
			}()
			s.server.WaitForShutdown()
		}()

		select {
		case <-done:
			s.logger.Debug("NATS server shutdown completed")
		case <-time.After(DefaultShutdownTimeout):
			s.logger.Warn("server shutdown timeout, shutdown may have completed with errors")
		}
	}
	s.stopped = true
	s.logger.Info("NATS server stopped")
	return nil
}

// IsRunning returns true if the server is running.
func (s *EmbeddedServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.stopped {
		return false
	}
	return s.server != nil && s.server.Running()
}

// URL returns the server URL.
func (s *EmbeddedServer) URL() string {
	if s.server != nil {
		return s.server.ClientURL()
	}
	return "nats://" + net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
}

// ClientURL returns the URL for clients to connect.
func (s *EmbeddedServer) ClientURL() string {
	return s.URL()
}

// Stats returns server statistics.
func (s *EmbeddedServer) Stats() *ServerStats {
	if s.server == nil {
		return nil
	}

	varz, _ := s.server.Varz(nil)
	jsz, _ := s.server.Jsz(nil)

	uptime, _ := time.ParseDuration(varz.Uptime)
	if uptime == 0 {
		// ensure positive uptime after start for flaky string formats
		uptime = time.Millisecond
	}

	// Safe conversion with bounds checking
	var totalMessages, totalBytes uint64
	// Use raw values for demonstration - actual conversion happens in helper function
	inMsgs := safeInt64ToUint64(varz.InMsgs)
	outMsgs := safeInt64ToUint64(varz.OutMsgs)
	totalMessages = inMsgs + outMsgs
	inBytes := safeInt64ToUint64(varz.InBytes)
	outBytes := safeInt64ToUint64(varz.OutBytes)
	totalBytes = inBytes + outBytes

	stats := &ServerStats{
		Connections:   varz.Connections,
		TotalMessages: totalMessages,
		TotalBytes:    totalBytes,
		Uptime:        uptime,
	}

	if jsz != nil {
		stats.JetStreamEnabled = true
		// Safe conversion with bounds checking for uint64 to int64
		stats.JetStreamMemory = safeUint64ToInt64(jsz.Memory)
		stats.JetStreamStorage = safeUint64ToInt64(jsz.Store)
	}

	s.logger.Debug("NATS server stats", "stats", stats)
	return stats
}

// WaitForShutdown waits for the server to shutdown.
func (s *EmbeddedServer) WaitForShutdown() {
	if s.server != nil {
		s.server.WaitForShutdown()
	}
}

// HealthCheck checks the health of the server.
func (s *EmbeddedServer) HealthCheck() error {
	if !s.IsRunning() {
		s.logger.Error("NATS server is not running")
		return errors.New("server is not running")
	}

	if !s.server.JetStreamEnabled() {
		s.logger.Error("JetStream not enabled in server configuration")
		return errors.New("JetStream not enabled in server configuration")
	}

	return nil
}

// ServerStats represents server statistics.
type ServerStats struct {
	Connections      int           `json:"connections"`
	TotalMessages    uint64        `json:"total_messages"`
	TotalBytes       uint64        `json:"total_bytes"`
	Uptime           time.Duration `json:"uptime"`
	JetStreamEnabled bool          `json:"jetstream_enabled"`
	JetStreamMemory  int64         `json:"jetstream_memory"`
	JetStreamStorage int64         `json:"jetstream_storage"`
}

// safeInt64ToUint64 safely converts int64 to uint64.
func safeInt64ToUint64(v int64) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}

// safeUint64ToInt64 safely converts uint64 to int64.
func safeUint64ToInt64(v uint64) int64 {
	if v > MaxInt63 {
		return MaxInt63
	}
	return int64(v)
}
