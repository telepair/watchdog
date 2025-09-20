package collector

import (
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/telepair/watchdog/internal/collector/system"
	"github.com/telepair/watchdog/pkg/natsx/client"
)

var (
	defaultAgentBucket   = "wd-agent"
	defaultAgentStream   = "wd-agent"
	defaultSubjectPrefix = "wd.a."
)

type Config struct {
	System             system.Config       `yaml:"system" json:"system"`
	AgentBucket        client.BucketConfig `yaml:"agent_bucket" json:"agent_bucket"`
	AgentStream        client.StreamConfig `yaml:"agent_stream" json:"agent_stream"`
	AgentSubjectPrefix string              `yaml:"agent_subject_prefix" json:"agent_subject_prefix"`
}

func DefaultConfig() Config {
	return Config{
		System: system.DefaultConfig(),
		AgentBucket: client.BucketConfig{
			Bucket:      defaultAgentBucket,
			History:     3,
			TTL:         7 * 24 * time.Hour,
			Storage:     jetstream.FileStorage,
			Replicas:    1,
			Compression: false,
		},
		AgentStream: client.StreamConfig{
			Name:       defaultAgentStream,
			Subjects:   []string{defaultSubjectPrefix + ">"},
			Retention:  jetstream.LimitsPolicy,
			MaxAge:     7 * 24 * time.Hour,
			MaxBytes:   1024 * 1024 * 1024,
			Storage:    jetstream.FileStorage,
			Replicas:   1,
			NoAck:      false,
			Duplicates: 5 * time.Minute,
		},
		AgentSubjectPrefix: defaultSubjectPrefix,
	}
}

func (c *Config) Parse() error {
	if err := c.System.Parse(); err != nil {
		return fmt.Errorf("failed to parse system config: %w", err)
	}
	if strings.TrimSpace(c.AgentBucket.Bucket) == "" {
		c.AgentBucket.Bucket = defaultAgentBucket
	}

	// Validate bucket name
	if err := client.ValidateBucketName(c.AgentBucket.Bucket); err != nil {
		return fmt.Errorf("invalid agent bucket name: %w", err)
	}

	if strings.TrimSpace(c.AgentStream.Name) == "" {
		c.AgentStream.Name = defaultAgentStream
	}
	if c.AgentStream.Subjects == nil {
		c.AgentStream.Subjects = []string{defaultSubjectPrefix + ">"}
	}

	// Validate all subjects
	for i, subject := range c.AgentStream.Subjects {
		if err := client.ValidateSubject(subject); err != nil {
			return fmt.Errorf("invalid subject at index %d: %w", i, err)
		}
	}

	if strings.TrimSpace(c.AgentSubjectPrefix) == "" {
		c.AgentSubjectPrefix = defaultSubjectPrefix
	}

	// Validate subject prefix (it should end with "." for pattern matching)
	if !strings.HasSuffix(c.AgentSubjectPrefix, ".") {
		c.AgentSubjectPrefix += "."
	}
	if err := client.ValidateSubject(c.AgentSubjectPrefix + "test"); err != nil {
		return fmt.Errorf("invalid agent subject prefix: %w", err)
	}

	return nil
}
