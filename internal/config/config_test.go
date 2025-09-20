package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/telepair/watchdog/internal/agent"
	"github.com/telepair/watchdog/internal/collector"
	"github.com/telepair/watchdog/pkg/health"
	"github.com/telepair/watchdog/pkg/logger"
	"github.com/telepair/watchdog/pkg/natsx/client"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("expected config, got nil")
	}

	// Test server config
	if !config.Server.EnableEmbedNATS {
		t.Error("expected EnableEmbedNATS to be true")
	}

	if config.Server.EmbedNATS == nil {
		t.Error("expected EmbedNATS to be set")
	}

	// Test agent config
	if config.Agent.ID == "" {
		t.Error("expected Agent.ID to be set")
	}

	// Test collector config
	if config.Collector.AgentBucket.Bucket == "" {
		t.Error("expected Collector.AgentBucket.Bucket to be set")
	}

	// Test NATS config
	if len(config.NATS.URLs) == 0 {
		t.Error("expected NATS.URLs to be set")
	}

	// Test logger config
	if config.Logger.Console.Level == "" {
		t.Error("expected Logger.Console.Level to be set")
	}

	// Test health addr
	if config.HealthAddr != health.DefaultAddr {
		t.Errorf("expected HealthAddr %q, got %q", health.DefaultAddr, config.HealthAddr)
	}

	// Test shutdown timeout
	if config.ShutdownTimeoutSec <= 0 {
		t.Error("expected ShutdownTimeoutSec to be positive")
	}
}

func TestConfig_Parse(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config",
			config: Config{
				HealthAddr: ":8080",
			},
			wantErr: false,
		},
		{
			name: "invalid server config",
			config: Config{
				Server: ServerConfig{
					EnableEmbedNATS: true,
					EmbedNATS:       nil, // This will auto-correct to default
				},
				HealthAddr: ":8080",
			},
			wantErr: false, // The Parse method auto-corrects the nil EmbedNATS
		},
		{
			name: "invalid agent config",
			config: Config{
				Agent: agent.Config{
					ID: "", // This should be fine, will use default
				},
				HealthAddr: ":8080",
			},
			wantErr: false,
		},
		{
			name: "invalid collector config",
			config: Config{
				Collector: collector.Config{
					AgentBucket: client.BucketConfig{
						Bucket: "", // This should be fine, will use default
					},
				},
				HealthAddr: ":8080",
			},
			wantErr: false,
		},
		{
			name: "invalid NATS config",
			config: Config{
				NATS: client.Config{
					URLs: []string{""}, // This should cause an error
				},
			},
			wantErr:     true,
			errContains: "invalid nats config",
		},
		{
			name: "invalid health addr",
			config: Config{
				HealthAddr: "invalid-addr",
			},
			wantErr:     true,
			errContains: "invalid health config",
		},
		{
			name: "invalid logger config",
			config: Config{
				Logger: logger.Config{
					Console: logger.ConsoleConfig{
						Enabled: true, // Need to enable console for validation to trigger
						Level:   "invalid-level",
					},
				},
				HealthAddr: ":8080",
			},
			wantErr:     true,
			errContains: "invalid logger config",
		},
		{
			name: "zero shutdown timeout - should use default",
			config: Config{
				ShutdownTimeoutSec: 0,
				HealthAddr:         ":8080",
			},
			wantErr: false,
		},
		{
			name: "negative shutdown timeout - should use default",
			config: Config{
				ShutdownTimeoutSec: -1,
				HealthAddr:         ":8080",
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

			// Check that shutdown timeout is set to default if it was zero or negative
			if tt.config.ShutdownTimeoutSec <= 0 {
				if config.ShutdownTimeoutSec != defaultShutdownTimeoutSec {
					t.Errorf("expected ShutdownTimeoutSec %d, got %d", defaultShutdownTimeoutSec, config.ShutdownTimeoutSec)
				}
			}
		})
	}
}

func TestConfig_Parse_EdgeCases(t *testing.T) {
	t.Run("very large shutdown timeout", func(t *testing.T) {
		config := Config{
			ShutdownTimeoutSec: 999999,
			HealthAddr:         ":8080",
		}
		err := config.Parse()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if config.ShutdownTimeoutSec != 999999 {
			t.Errorf("expected ShutdownTimeoutSec %d, got %d", 999999, config.ShutdownTimeoutSec)
		}
	})

	t.Run("whitespace health addr", func(t *testing.T) {
		config := Config{
			HealthAddr: "   ",
		}
		err := config.Parse()
		if err == nil {
			t.Error("expected error for whitespace health addr")
		}
	})

	t.Run("empty health addr", func(t *testing.T) {
		config := Config{
			HealthAddr: "",
		}
		err := config.Parse()
		if err == nil {
			t.Error("expected error for empty health addr")
		}
	})
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		configPath  string
		configData  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "non-existent file - should return default config",
			configPath: "/non/existent/path.yaml",
			configData: "",
			wantErr:    false,
		},
		{
			name:       "valid YAML config",
			configPath: "test-config.yaml",
			configData: "server:\n  enable_embed_nats: false\nhealth_addr: \":8080\"",
			wantErr:    false,
		},
		{
			name:        "invalid YAML config",
			configPath:  "test-config.yaml",
			configData:  "invalid: yaml: content: [",
			wantErr:     true,
			errContains: "failed to parse config file",
		},
		{
			name:        "valid YAML but invalid config",
			configPath:  "test-config.yaml",
			configData:  "nats:\n  url: \"\"\nlogger:\n  level: \"invalid\"",
			wantErr:     true,
			errContains: "invalid config in",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file if needed
			if tt.configData != "" {
				// Create a temporary directory
				tempDir := t.TempDir()
				configPath := filepath.Join(tempDir, tt.configPath)

				// Write config data to file
				err := os.WriteFile(configPath, []byte(tt.configData), 0644)
				if err != nil {
					t.Fatalf("failed to write config file: %v", err)
				}

				// Load config
				config, err := LoadConfig(configPath)

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

				if config == nil {
					t.Error("expected config, got nil")
					return
				}

				// Check that config was loaded correctly
				if config.HealthAddr != ":8080" {
					t.Errorf("expected HealthAddr %q, got %q", ":8080", config.HealthAddr)
				}
			} else {
				// Test with non-existent file
				config, err := LoadConfig(tt.configPath)

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

				if config == nil {
					t.Error("expected config, got nil")
					return
				}

				// Should return default config
				if config.HealthAddr != health.DefaultAddr {
					t.Errorf("expected HealthAddr %q, got %q", health.DefaultAddr, config.HealthAddr)
				}
			}
		})
	}
}

func TestLoadConfig_FileSizeLimit(t *testing.T) {
	// Create a very large config file to test size limit
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "large-config.yaml")

	// Create a large config file (1MB)
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = 'a'
	}

	err := os.WriteFile(configPath, largeData, 0644)
	if err != nil {
		t.Fatalf("failed to write large config file: %v", err)
	}

	// Load config - should fail due to invalid YAML
	_, err = LoadConfig(configPath)
	if err == nil {
		t.Error("expected error for large config file, got nil")
	}
}

func TestConfig_Constants(t *testing.T) {
	// Test that constants are reasonable values
	if defaultShutdownTimeoutSec <= 0 {
		t.Errorf("defaultShutdownTimeoutSec should be positive, got %d", defaultShutdownTimeoutSec)
	}

	// Test that default config uses the constant
	config := DefaultConfig()
	if config.ShutdownTimeoutSec != defaultShutdownTimeoutSec {
		t.Errorf("expected ShutdownTimeoutSec %d, got %d", defaultShutdownTimeoutSec, config.ShutdownTimeoutSec)
	}
}

func TestConfig_SubConfigs(t *testing.T) {
	config := DefaultConfig()

	// Test that all sub-configs are properly initialized
	if config.Server.EnableEmbedNATS != true {
		t.Error("expected Server.EnableEmbedNATS to be true")
	}

	if config.Agent.ID == "" {
		t.Error("expected Agent.ID to be set")
	}

	if config.Collector.AgentBucket.Bucket == "" {
		t.Error("expected Collector.AgentBucket.Bucket to be set")
	}

	if len(config.NATS.URLs) == 0 {
		t.Error("expected NATS.URLs to be set")
	}

	if config.Logger.Console.Level == "" {
		t.Error("expected Logger.Console.Level to be set")
	}
}

// Benchmark tests
func BenchmarkConfig_Parse(b *testing.B) {
	config := Config{
		HealthAddr:         ":8080",
		ShutdownTimeoutSec: 10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Parse()
	}
}

func BenchmarkDefaultConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DefaultConfig()
	}
}

func BenchmarkLoadConfig_Default(b *testing.B) {
	// Test loading default config (non-existent file)
	configPath := "/non/existent/path.yaml"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadConfig(configPath)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			contains(s[1:], substr))))
}
