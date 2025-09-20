package config

import (
	"fmt"

	"github.com/telepair/watchdog/pkg/natsx/embed"
)

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	EnableEmbedNATS bool                `yaml:"enable_embed_nats" json:"enable_embed_nats"`
	EmbedNATS       *embed.ServerConfig `yaml:"embed_nats" json:"embed_nats"`
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		EnableEmbedNATS: true,
		EmbedNATS:       embed.DefaultServerConfig(),
	}
}

func (s *ServerConfig) Parse() error {
	if s.EnableEmbedNATS && s.EmbedNATS == nil {
		s.EmbedNATS = embed.DefaultServerConfig()
	}
	if s.EnableEmbedNATS {
		if err := s.EmbedNATS.Validate(); err != nil {
			return fmt.Errorf("invalid embed_nats config: %w", err)
		}
	}
	return nil
}
