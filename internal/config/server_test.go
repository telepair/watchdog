package config

import (
	"testing"

	"github.com/telepair/watchdog/pkg/natsx/embed"
)

func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	if !config.EnableEmbedNATS {
		t.Error("expected EnableEmbedNATS to be true")
	}

	if config.EmbedNATS == nil {
		t.Error("expected EmbedNATS to be set")
	}

	// Test that embed NATS config is valid
	if err := config.EmbedNATS.Validate(); err != nil {
		t.Errorf("expected EmbedNATS config to be valid: %v", err)
	}
}

func TestServerConfig_Parse(t *testing.T) {
	tests := []struct {
		name        string
		config      ServerConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config with embedded NATS enabled",
			config: ServerConfig{
				EnableEmbedNATS: true,
				EmbedNATS:       embed.DefaultServerConfig(),
			},
			wantErr: false,
		},
		{
			name: "valid config with embedded NATS disabled",
			config: ServerConfig{
				EnableEmbedNATS: false,
				EmbedNATS:       nil,
			},
			wantErr: false,
		},
		{
			name: "embedded NATS enabled but nil config - should use default",
			config: ServerConfig{
				EnableEmbedNATS: true,
				EmbedNATS:       nil,
			},
			wantErr: false,
		},
		{
			name: "embedded NATS enabled with invalid config",
			config: ServerConfig{
				EnableEmbedNATS: true,
				EmbedNATS: &embed.ServerConfig{
					Port: -1, // Invalid port, but will be auto-corrected
				},
			},
			wantErr: false, // The Validate method auto-corrects invalid values
		},
		{
			name: "embedded NATS disabled with invalid config - should not validate",
			config: ServerConfig{
				EnableEmbedNATS: false,
				EmbedNATS: &embed.ServerConfig{
					Port: -1, // Invalid port, but should be ignored
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.config
			err := config.Parse()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check that embed NATS config is set when enabled
			if config.EnableEmbedNATS && config.EmbedNATS == nil {
				t.Error("expected EmbedNATS to be set when EnableEmbedNATS is true")
			}
		})
	}
}

func TestServerConfig_Parse_EdgeCases(t *testing.T) {
	t.Run("embedded NATS enabled with zero value config", func(t *testing.T) {
		config := ServerConfig{
			EnableEmbedNATS: true,
			EmbedNATS:       &embed.ServerConfig{}, // Zero value
		}
		err := config.Parse()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("embedded NATS disabled with zero value config", func(t *testing.T) {
		config := ServerConfig{
			EnableEmbedNATS: false,
			EmbedNATS:       &embed.ServerConfig{}, // Zero value, should be ignored
		}
		err := config.Parse()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("embedded NATS enabled with custom config", func(t *testing.T) {
		config := ServerConfig{
			EnableEmbedNATS: true,
			EmbedNATS: &embed.ServerConfig{
				Port: 4222,
				Host: "localhost",
			},
		}
		err := config.Parse()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestServerConfig_DefaultValues(t *testing.T) {
	config := DefaultServerConfig()

	// Test that default values are reasonable
	if !config.EnableEmbedNATS {
		t.Error("expected EnableEmbedNATS to be true by default")
	}

	if config.EmbedNATS == nil {
		t.Error("expected EmbedNATS to be set by default")
	}

	// Test that the embed NATS config is valid
	if err := config.EmbedNATS.Validate(); err != nil {
		t.Errorf("expected default EmbedNATS config to be valid: %v", err)
	}
}

func TestServerConfig_EmbedNATSConfig(t *testing.T) {
	config := DefaultServerConfig()

	// Test that embed NATS config has reasonable values
	if config.EmbedNATS.Port <= 0 {
		t.Error("expected EmbedNATS.Port to be positive")
	}

	if config.EmbedNATS.Host == "" {
		t.Error("expected EmbedNATS.Host to be set")
	}

	// Test that the config can be validated
	if err := config.EmbedNATS.Validate(); err != nil {
		t.Errorf("expected EmbedNATS config to be valid: %v", err)
	}
}

func TestServerConfig_EnableEmbedNATS(t *testing.T) {
	tests := []struct {
		name            string
		enableEmbedNATS bool
		embedNATS       *embed.ServerConfig
		expectedNil     bool
	}{
		{
			name:            "enabled with config",
			enableEmbedNATS: true,
			embedNATS:       embed.DefaultServerConfig(),
			expectedNil:     false,
		},
		{
			name:            "enabled without config",
			enableEmbedNATS: true,
			embedNATS:       nil,
			expectedNil:     false, // Should be set to default after Parse
		},
		{
			name:            "disabled with config",
			enableEmbedNATS: false,
			embedNATS:       embed.DefaultServerConfig(),
			expectedNil:     false, // Config should remain as is
		},
		{
			name:            "disabled without config",
			enableEmbedNATS: false,
			embedNATS:       nil,
			expectedNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ServerConfig{
				EnableEmbedNATS: tt.enableEmbedNATS,
				EmbedNATS:       tt.embedNATS,
			}

			err := config.Parse()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.expectedNil && config.EmbedNATS != nil {
				t.Error("expected EmbedNATS to be nil")
			}

			if !tt.expectedNil && config.EmbedNATS == nil {
				t.Error("expected EmbedNATS to be set")
			}
		})
	}
}

// Benchmark tests
func BenchmarkServerConfig_Parse(b *testing.B) {
	config := ServerConfig{
		EnableEmbedNATS: true,
		EmbedNATS:       embed.DefaultServerConfig(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Parse()
	}
}

func BenchmarkDefaultServerConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DefaultServerConfig()
	}
}
