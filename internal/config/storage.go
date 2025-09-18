package config

import (
	"fmt"
	"strings"
	"time"
)

var (
	defaultAgentBucketName     = "wd-agents"
	defaultAgentBucketHistory  = uint8(3)
	defaultAgentBucketTTL      = 24 * time.Hour
	defaultAgentBucketStorage  = "file"
	defaultAgentBucketReplicas = 1

	defaultAgentStreamName           = "wd-agents"
	defaultAgentStreamDescription    = "Agent communications stream"
	defaultAgentStreamSubjectPattern = "wd.a.>"
	defaultAgentStreamRetention      = "limits"
	defaultAgentStreamMaxAge         = 7 * 24 * time.Hour       // 7 days
	defaultAgentStreamMaxBytes       = int64(100 * 1024 * 1024) // 100MB
	defaultAgentStreamMaxMsgs        = int64(100000)            // 100k messages
	defaultAgentStreamStorage        = "file"
	defaultAgentStreamReplicas       = 1
	defaultAgentStreamDuplicates     = 5 * time.Minute
)

type StorageConfig struct {
	AgentBucket AgentBucketConfig `yaml:"agent_bucket" json:"agent_bucket" mapstructure:"agent_bucket"`
	AgentStream AgentStreamConfig `yaml:"agent_stream" json:"agent_stream" mapstructure:"agent_stream"`
}

func DefaultStorageConfig() StorageConfig {
	return StorageConfig{
		AgentBucket: DefaultAgentBucketConfig(),
		AgentStream: DefaultAgentStreamConfig(),
	}
}

func (c *StorageConfig) Validate() error {
	if err := c.AgentBucket.Validate(); err != nil {
		return fmt.Errorf("invalid agent bucket config: %w", err)
	}
	if err := c.AgentStream.Validate(); err != nil {
		return fmt.Errorf("invalid agent stream config: %w", err)
	}
	return nil
}

func (c *StorageConfig) SetDefaults() {
	c.AgentBucket.SetDefaults()
	c.AgentStream.SetDefaults()
}

// AgentBucketConfig configures the NATS KV bucket for agent data storage
type AgentBucketConfig struct {
	Name        string        `yaml:"name"  mapstructure:"name"`
	History     uint8         `yaml:"history" mapstructure:"history"`
	TTL         time.Duration `yaml:"ttl" mapstructure:"ttl"`
	Storage     string        `yaml:"storage" mapstructure:"storage"`
	Replicas    int           `yaml:"replicas" mapstructure:"replicas"`
	Compression bool          `yaml:"compression" mapstructure:"compression"`
}

// DefaultAgentBucketConfig returns default configuration for agent KV bucket
func DefaultAgentBucketConfig() AgentBucketConfig {
	return AgentBucketConfig{
		Name:        defaultAgentBucketName,
		History:     defaultAgentBucketHistory,
		TTL:         defaultAgentBucketTTL,
		Storage:     defaultAgentBucketStorage,
		Replicas:    defaultAgentBucketReplicas,
		Compression: false,
	}
}

// Validate validates the agent bucket configuration
func (c *AgentBucketConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("bucket name cannot be empty")
	}
	if c.History == 0 {
		return fmt.Errorf("history must be greater than 0")
	}
	if c.TTL <= 0 {
		return fmt.Errorf("TTL must be greater than 0")
	}
	if c.Storage != "file" && c.Storage != "memory" {
		return fmt.Errorf("storage must be 'file' or 'memory', got %s", c.Storage)
	}
	if c.Replicas < 1 {
		return fmt.Errorf("replicas must be at least 1")
	}
	return nil
}

// SetDefaults sets default values for agent bucket configuration
func (c *AgentBucketConfig) SetDefaults() {
	if c.Name == "" {
		c.Name = defaultAgentBucketName
	}
	if c.History == 0 {
		c.History = defaultAgentBucketHistory
	}
	if c.TTL == 0 {
		c.TTL = defaultAgentBucketTTL
	}
	if c.Storage == "" {
		c.Storage = defaultAgentBucketStorage
	}
	if c.Replicas == 0 {
		c.Replicas = defaultAgentBucketReplicas
	}
}

// BucketName returns the bucket name
func (c *AgentBucketConfig) BucketName() string {
	return c.Name
}

// InfoKey returns the info key pattern for the bucket
func (c *AgentBucketConfig) InfoKey(agentID string) string {
	return fmt.Sprintf("info.%s", agentID)
}

// ConfigKey returns the config key pattern for the bucket
func (c *AgentBucketConfig) ConfigKey(agentID string) string {
	return fmt.Sprintf("config.%s", agentID)
}

// StatusKey returns the status key pattern for the bucket
func (c *AgentBucketConfig) StatusKey(agentID string) string {
	return fmt.Sprintf("status.%s", agentID)
}

// AgentStreamConfig configures the NATS JetStream for agent communications
type AgentStreamConfig struct {
	Name           string        `yaml:"name" mapstructure:"name"`
	Description    string        `yaml:"description" mapstructure:"description"`
	SubjectPattern string        `yaml:"subject_pattern" mapstructure:"subject_pattern"`
	Retention      string        `yaml:"retention" mapstructure:"retention"`
	MaxAge         time.Duration `yaml:"max_age" mapstructure:"max_age"`
	MaxBytes       int64         `yaml:"max_bytes" mapstructure:"max_bytes"`
	MaxMsgs        int64         `yaml:"max_msgs" mapstructure:"max_msgs"`
	Storage        string        `yaml:"storage" mapstructure:"storage"`
	Replicas       int           `yaml:"replicas" mapstructure:"replicas"`
	NoAck          bool          `yaml:"no_ack" mapstructure:"no_ack"`
	Duplicates     time.Duration `yaml:"duplicates" mapstructure:"duplicates"`

	subjectPrefix string `yaml:"-" mapstructure:"-"`
}

// DefaultAgentStreamConfig returns default configuration for agent stream
func DefaultAgentStreamConfig() AgentStreamConfig {
	return AgentStreamConfig{
		Name:           defaultAgentStreamName,
		Description:    defaultAgentStreamDescription,
		SubjectPattern: defaultAgentStreamSubjectPattern,
		Retention:      defaultAgentStreamRetention,
		MaxAge:         defaultAgentStreamMaxAge,
		MaxBytes:       defaultAgentStreamMaxBytes,
		MaxMsgs:        defaultAgentStreamMaxMsgs,
		Storage:        defaultAgentStreamStorage,
		Replicas:       defaultAgentStreamReplicas,
		NoAck:          false,
		Duplicates:     defaultAgentStreamDuplicates,
	}
}

// Validate validates the agent stream configuration
func (c *AgentStreamConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("stream name cannot be empty")
	}
	if len(c.SubjectPattern) < 2 || c.SubjectPattern[len(c.SubjectPattern)-2:] != ".>" {
		return fmt.Errorf("subject pattern must be non-empty and end with '>'")
	}
	if c.Retention != "limits" && c.Retention != "interest" && c.Retention != "workqueue" {
		return fmt.Errorf("retention must be 'limits', 'interest', or 'workqueue', got %s", c.Retention)
	}
	if c.MaxAge <= 0 {
		return fmt.Errorf("max age must be greater than 0")
	}
	if c.MaxBytes <= 0 {
		return fmt.Errorf("max bytes must be greater than 0")
	}
	if c.MaxMsgs <= 0 {
		return fmt.Errorf("max messages must be greater than 0")
	}
	if c.Storage != "file" && c.Storage != "memory" {
		return fmt.Errorf("storage must be 'file' or 'memory', got %s", c.Storage)
	}
	if c.Replicas < 1 {
		return fmt.Errorf("replicas must be at least 1")
	}
	if c.Duplicates < 0 {
		return fmt.Errorf("duplicates window cannot be negative")
	}
	c.subjectPrefix = strings.TrimRight(c.SubjectPattern, ".>")
	return nil
}

// SetDefaults sets default values for agent stream configuration
func (c *AgentStreamConfig) SetDefaults() {
	if c.Name == "" {
		c.Name = defaultAgentStreamName
	}
	if c.Description == "" {
		c.Description = defaultAgentStreamDescription
	}
	if c.SubjectPattern == "" {
		c.SubjectPattern = defaultAgentStreamSubjectPattern
	}
	if c.Retention == "" {
		c.Retention = defaultAgentStreamRetention
	}
	if c.MaxAge == 0 {
		c.MaxAge = defaultAgentStreamMaxAge
	}
	if c.MaxBytes == 0 {
		c.MaxBytes = defaultAgentStreamMaxBytes
	}
	if c.MaxMsgs == 0 {
		c.MaxMsgs = defaultAgentStreamMaxMsgs
	}
	if c.Storage == "" {
		c.Storage = defaultAgentStreamStorage
	}
	if c.Replicas == 0 {
		c.Replicas = defaultAgentStreamReplicas
	}
	if c.Duplicates == 0 {
		c.Duplicates = defaultAgentStreamDuplicates
	}
}

// StreamName returns the stream name
func (c *AgentStreamConfig) StreamName() string {
	return c.Name
}

func (c *AgentStreamConfig) MailboxSubject(agentID string) string {
	return c.subjectPrefix + agentID + ".mbox"
}

func (c *AgentStreamConfig) WarnSubject(agentID string) string {
	return c.subjectPrefix + agentID + ".warn"
}

func (c *AgentStreamConfig) ErrorSubject(agentID string) string {
	return c.subjectPrefix + agentID + ".error"
}

func (c *AgentStreamConfig) EventSubjectPattern(agentID string) string {
	return c.subjectPrefix + agentID + ".event.*"
}

func (c *AgentStreamConfig) EventSubject(agentID, eventType string) string {
	return c.subjectPrefix + agentID + ".event." + eventType
}

func (c *AgentStreamConfig) SysSubjectPattern(agentID string) string {
	return c.subjectPrefix + agentID + ".sys.*"
}

func (c *AgentStreamConfig) SysInfoSubject(agentID string) string {
	return c.subjectPrefix + agentID + ".sys.info"
}

func (c *AgentStreamConfig) SysCPUSubject(agentID string) string {
	return c.subjectPrefix + agentID + ".sys.cpu"
}

func (c *AgentStreamConfig) SysMemorySubject(agentID string) string {
	return c.subjectPrefix + agentID + ".sys.mem"
}

func (c *AgentStreamConfig) SysDiskSubject(agentID string) string {
	return c.subjectPrefix + agentID + ".sys.disk"
}

func (c *AgentStreamConfig) SysNetworkSubject(agentID string) string {
	return c.subjectPrefix + agentID + ".sys.network"
}

func (c *AgentStreamConfig) SysLoadSubject(agentID string) string {
	return c.subjectPrefix + agentID + ".sys.load"
}

func (c *AgentStreamConfig) SysUptimeSubject(agentID string) string {
	return c.subjectPrefix + agentID + ".sys.uptime"
}

func (c *AgentStreamConfig) ExecSubjectPattern(agentID string) string {
	return c.subjectPrefix + agentID + ".exec.*.*"
}

func (c *AgentStreamConfig) ExecSubject(agentID, execType, id string) string {
	return c.subjectPrefix + agentID + ".exec." + execType + "." + id
}

func (c *AgentStreamConfig) ExecResultSubject(agentID, execType, id string) string {
	return c.subjectPrefix + agentID + ".exec." + execType + "." + id + ".result"
}
