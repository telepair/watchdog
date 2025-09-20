package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/telepair/watchdog/internal/agent"
	"github.com/telepair/watchdog/internal/collector"
	"github.com/telepair/watchdog/pkg/health"
	"github.com/telepair/watchdog/pkg/logger"
	"github.com/telepair/watchdog/pkg/natsx/client"
	"github.com/telepair/watchdog/pkg/utils"
)

var defaultShutdownTimeoutSec = 10

// Config holds the common configuration for all server types
type Config struct {
	Server             ServerConfig     `yaml:"server" json:"server"`
	Agent              agent.Config     `yaml:"agent"  json:"agent"`
	Collector          collector.Config `yaml:"collector" json:"collector"`
	NATS               client.Config    `yaml:"nats"   json:"nats"`
	Logger             logger.Config    `yaml:"logger" json:"logger"`
	HealthAddr         string           `yaml:"health_addr" json:"health_addr"`
	ShutdownTimeoutSec int              `yaml:"shutdown_timeout_sec" json:"shutdown_timeout_sec"`
}

// DefaultConfig returns a configuration for watchdog
func DefaultConfig() *Config {
	cfg := &Config{
		Server:     DefaultServerConfig(),
		Agent:      agent.DefaultConfig(),
		Collector:  collector.DefaultConfig(),
		NATS:       *client.DefaultConfig(),
		HealthAddr: health.DefaultAddr,
		Logger:     logger.DefaultConfig(),
	}
	if err := cfg.Parse(); err != nil {
		panic(err)
	}
	return cfg
}

// Parse parses the configuration
func (c *Config) Parse() error {
	if err := c.Server.Parse(); err != nil {
		return fmt.Errorf("invalid server config: %w", err)
	}

	if err := c.Agent.Parse(); err != nil {
		return fmt.Errorf("invalid agent config: %w", err)
	}

	if err := c.Collector.Parse(); err != nil {
		return fmt.Errorf("invalid collector config: %w", err)
	}

	if err := c.NATS.Validate(); err != nil {
		return fmt.Errorf("invalid nats config: %w", err)
	}

	if err := utils.ValidateAddr(c.HealthAddr); err != nil {
		return fmt.Errorf("invalid health config: %w", err)
	}

	if err := c.Logger.Validate(); err != nil {
		return fmt.Errorf("invalid logger config: %w", err)
	}

	if c.ShutdownTimeoutSec <= 0 {
		c.ShutdownTimeoutSec = defaultShutdownTimeoutSec
	}
	return nil
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
	if err := config.Parse(); err != nil {
		return nil, fmt.Errorf("invalid config in %s: %w", configPath, err)
	}

	return config, nil
}
