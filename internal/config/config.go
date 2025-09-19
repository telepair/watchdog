package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/telepair/watchdog/pkg/health"
	"github.com/telepair/watchdog/pkg/logger"
	"github.com/telepair/watchdog/pkg/natsx/client"
	"github.com/telepair/watchdog/pkg/utils"
)

var defaultShutdownTimeoutSec = 10

// Common event types
const (
	EventTypeStartup    = "startup"
	EventTypeShutdown   = "shutdown"
	EventTypeRestart    = "restart"
	EventTypeConfigured = "configured"
	EventTypeHeartbeat  = "heartbeat"
)

// Common execution types
const (
	ExecTypeCommand = "cmd"
	ExecTypeScript  = "script"
	ExecTypeTask    = "task"
	ExecTypeJob     = "job"
)

// Config holds the common configuration for all server types
type Config struct {
	Server             ServerConfig  `yaml:"server" json:"server"`
	Agent              AgentConfig   `yaml:"agent"  json:"agent"`
	Storage            StorageConfig `yaml:"storage" json:"storage"`
	NATS               client.Config `yaml:"nats"   json:"nats"`
	Health             health.Config `yaml:"health" json:"health"`
	Logger             logger.Config `yaml:"logger" json:"logger"`
	ShutdownTimeoutSec int           `yaml:"shutdown_timeout_sec" json:"shutdown_timeout_sec"`
}

// DefaultConfig returns a configuration for watchdog
func DefaultConfig() *Config {
	cfg := &Config{
		Server:  DefaultServerConfig(),
		Agent:   DefaultAgentConfig(),
		Storage: DefaultStorageConfig(),
		NATS:    *client.DefaultConfig(),
		Health:  *health.DefaultConfig(),
		Logger:  logger.DefaultConfig(),
	}
	cfg.SetDefaults()
	return cfg
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("invalid server config: %w", err)
	}

	if err := c.Agent.Validate(); err != nil {
		return fmt.Errorf("invalid agent config: %w", err)
	}

	if err := c.Storage.Validate(); err != nil {
		return fmt.Errorf("invalid storage config: %w", err)
	}

	if err := c.NATS.Validate(); err != nil {
		return fmt.Errorf("invalid nats config: %w", err)
	}

	if err := c.Health.Parse(); err != nil {
		return fmt.Errorf("invalid health config: %w", err)
	}

	if err := c.Logger.Validate(); err != nil {
		return fmt.Errorf("invalid logger config: %w", err)
	}

	return nil
}

// SetDefaults sets default values for the configuration
func (c *Config) SetDefaults() {
	c.Server.SetDefaults()
	c.Agent.SetDefaults()
	c.Storage.SetDefaults()
	if c.ShutdownTimeoutSec <= 0 {
		c.ShutdownTimeoutSec = defaultShutdownTimeoutSec
	}
}

// LoadConfig loads configuration from a file, or returns default config if file doesn't exist
func LoadConfig(configPath string) (*Config, error) {
	configPath, err := utils.ExpandPath(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand config path: %w", err)
	}

	// If config file doesn't exist, return default agent config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	// Read config file with size limit to prevent potential DoS
	// #nosec G304 -- configPath is controlled by user via command line flag
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Parse YAML
	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Apply defaults and validate
	config.SetDefaults()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config in %s: %w", configPath, err)
	}

	return config, nil
}
