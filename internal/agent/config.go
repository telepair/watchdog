package agent

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	defaultID = "watchdog-agent"

	defaultReportInterval    = 600
	defaultHeartbeatInterval = 5

	// validAgentIDPattern matches valid agent ID characters for NATS subject segments
	// Only alphanumeric, hyphens, and underscores allowed (no dots to avoid subject confusion)
	validAgentIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// AgentConfig holds agent-specific configuration
type Config struct {
	ID                string `yaml:"id" json:"id"`
	ReportInterval    int    `yaml:"info_report_interval" json:"info_report_interval"`
	HeartbeatInterval int    `yaml:"heartbeat_interval" json:"heartbeat_interval"`
}

func DefaultConfig() Config {
	return Config{
		ID:                defaultID,
		ReportInterval:    defaultReportInterval,
		HeartbeatInterval: defaultHeartbeatInterval,
	}
}

func (c *Config) Parse() error {
	if strings.TrimSpace(c.ID) == "" {
		c.ID = defaultID
	}

	if err := validateAgentID(c.ID); err != nil {
		return fmt.Errorf("invalid agent ID: %w", err)
	}

	if c.ReportInterval <= 0 {
		c.ReportInterval = defaultReportInterval
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = defaultHeartbeatInterval
	}
	return nil
}

// validateAgentID validates that agent ID is suitable as NATS subject segment
func validateAgentID(id string) error {
	if id == "" {
		return fmt.Errorf("agent ID cannot be empty")
	}

	if len(id) > 63 {
		return fmt.Errorf("agent ID too long (max 63 characters)")
	}

	if !validAgentIDPattern.MatchString(id) {
		return fmt.Errorf("agent ID contains invalid characters, only alphanumeric, hyphens and underscores are allowed")
	}

	if strings.HasPrefix(id, "-") || strings.HasSuffix(id, "-") {
		return fmt.Errorf("agent ID cannot start or end with hyphen")
	}

	if strings.Contains(id, "--") {
		return fmt.Errorf("agent ID cannot contain consecutive hyphens")
	}

	return nil
}
