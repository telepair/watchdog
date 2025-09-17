package client

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/nats-io/nats.go"
)

// GlobalConnectionManager manages a global NATS client connection.
type GlobalConnectionManager struct {
	client      *Client
	config      *Config
	mu          sync.RWMutex
	initialized bool
	logger      *slog.Logger
}

//nolint:gochecknoglobals // globalManager is a singleton pattern for global connection management
var (
	globalManager = &GlobalConnectionManager{}
)

// SetGlobalConfig sets the configuration for the global NATS client.
func SetGlobalConfig(config *Config) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	// If already initialized with different config, close existing client
	if globalManager.initialized && globalManager.client != nil {
		if err := globalManager.client.Close(); err != nil {
			// Log error but continue with reconfiguration
			slog.Default().Warn("failed to close existing client during reconfiguration", "error", err)
		}
		globalManager.client = nil
		globalManager.initialized = false
	}

	globalManager.config = config
	return nil
}

// GetGlobalClient returns the global NATS client instance
// Creates and initializes the client if not already done.
func GetGlobalClient() (*Client, error) {
	globalManager.mu.RLock()
	if globalManager.client != nil && globalManager.client.IsConnected() {
		defer globalManager.mu.RUnlock()
		return globalManager.client, nil
	}
	globalManager.mu.RUnlock()

	return globalManager.initClient()
}

// initClient initializes the global client with proper locking.
func (m *GlobalConnectionManager) initClient() (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check pattern - another goroutine might have initialized
	if m.client != nil && m.client.IsConnected() {
		return m.client, nil
	}

	// Use default config if none set
	if m.config == nil {
		m.config = DefaultConfig()
	}

	// Initialize logger once
	if m.logger == nil {
		m.logger = slog.Default().With("component", "natsx.gcm")
	}

	// Close existing client if any
	if m.client != nil {
		if err := m.client.Close(); err != nil {
			// Log error but continue with initialization
			m.logger.Warn("failed to close existing client during reinitialization", "error", err)
		}
	}

	// Create new client
	client, err := NewClient(m.config)
	if err != nil {
		m.logger.Error("failed to create global NATS client", "error", err)
		// Preserve last known client if exists; return error
		return nil, fmt.Errorf("failed to create global client: %w", err)
	}

	// Validate connection after creation
	if !client.IsConnected() {
		if closeErr := client.Close(); closeErr != nil {
			m.logger.Warn("failed to close client after connection validation failure", "error", closeErr)
		}
		m.logger.Error("failed to establish connection to NATS")
		return nil, errors.New("failed to establish connection to NATS")
	}

	m.client = client
	m.initialized = true

	m.logger.Info("global NATS client initialized",
		"urls", m.config.URLs,
	)

	return m.client, nil
}

// IsGlobalClientConnected checks if the global client is connected.
func IsGlobalClientConnected() bool {
	globalManager.mu.RLock()
	defer globalManager.mu.RUnlock()

	return globalManager.client != nil && globalManager.client.IsConnected()
}

// CloseGlobalClient closes the global client connection.
func CloseGlobalClient() error {
	globalManager.mu.Lock()
	defer globalManager.mu.Unlock()

	if globalManager.client != nil {
		if err := globalManager.client.Close(); err != nil {
			return err
		}
		globalManager.client = nil
		globalManager.initialized = false

		if globalManager.logger != nil {
			globalManager.logger.Info("global NATS client closed")
		}
	}

	return nil
}

// GlobalClientStats returns statistics about the global client.
type GlobalClientStats struct {
	IsConnected   bool     `json:"is_connected"`
	Subscriptions int      `json:"subscriptions"`
	ServerURL     string   `json:"server_url,omitempty"`
	ConfigURLs    []string `json:"config_urls,omitempty"`
}

// GetGlobalClientStats returns statistics about the global client.
func GetGlobalClientStats() *GlobalClientStats {
	globalManager.mu.RLock()
	defer globalManager.mu.RUnlock()

	stats := &GlobalClientStats{}

	if globalManager.config != nil {
		stats.ConfigURLs = globalManager.config.URLs
	}

	if globalManager.client != nil {
		stats.IsConnected = globalManager.client.IsConnected()
		stats.Subscriptions = globalManager.client.GetSubscriptionCount()

		if globalManager.client.conn != nil {
			stats.ServerURL = globalManager.client.conn.ConnectedUrl()
		}
	}

	return stats
}

// GetConnection returns the underlying NATS connection from the global client.
func GetConnection() *nats.Conn {
	globalManager.mu.RLock()
	defer globalManager.mu.RUnlock()

	if globalManager.client != nil && globalManager.client.IsConnected() {
		return globalManager.client.conn
	}
	return nil
}
