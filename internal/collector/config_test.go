package collector

import (
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/telepair/watchdog/pkg/natsx/client"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test system config
	if config.System.GlobalInterval <= 0 {
		t.Error("expected GlobalInterval to be positive")
	}

	// Test agent bucket config
	if config.AgentBucket.Bucket != defaultAgentBucket {
		t.Errorf("expected AgentBucket.Bucket %q, got %q", defaultAgentBucket, config.AgentBucket.Bucket)
	}

	if config.AgentBucket.History != 3 {
		t.Errorf("expected AgentBucket.History %d, got %d", 3, config.AgentBucket.History)
	}

	if config.AgentBucket.TTL != 7*24*time.Hour {
		t.Errorf("expected AgentBucket.TTL %v, got %v", 7*24*time.Hour, config.AgentBucket.TTL)
	}

	if config.AgentBucket.Storage != jetstream.FileStorage {
		t.Errorf("expected AgentBucket.Storage %v, got %v", jetstream.FileStorage, config.AgentBucket.Storage)
	}

	if config.AgentBucket.Replicas != 1 {
		t.Errorf("expected AgentBucket.Replicas %d, got %d", 1, config.AgentBucket.Replicas)
	}

	if config.AgentBucket.Compression != false {
		t.Errorf("expected AgentBucket.Compression %v, got %v", false, config.AgentBucket.Compression)
	}

	// Test agent stream config
	if config.AgentStream.Name != defaultAgentStream {
		t.Errorf("expected AgentStream.Name %q, got %q", defaultAgentStream, config.AgentStream.Name)
	}

	expectedSubjects := []string{defaultSubjectPrefix + ">"}
	if len(config.AgentStream.Subjects) != len(expectedSubjects) {
		t.Errorf("expected AgentStream.Subjects %v, got %v", expectedSubjects, config.AgentStream.Subjects)
	}

	if config.AgentStream.Retention != jetstream.LimitsPolicy {
		t.Errorf("expected AgentStream.Retention %v, got %v", jetstream.LimitsPolicy, config.AgentStream.Retention)
	}

	if config.AgentStream.MaxAge != 7*24*time.Hour {
		t.Errorf("expected AgentStream.MaxAge %v, got %v", 7*24*time.Hour, config.AgentStream.MaxAge)
	}

	if config.AgentStream.MaxBytes != 1024*1024*1024 {
		t.Errorf("expected AgentStream.MaxBytes %d, got %d", 1024*1024*1024, config.AgentStream.MaxBytes)
	}

	if config.AgentStream.Storage != jetstream.FileStorage {
		t.Errorf("expected AgentStream.Storage %v, got %v", jetstream.FileStorage, config.AgentStream.Storage)
	}

	if config.AgentStream.Replicas != 1 {
		t.Errorf("expected AgentStream.Replicas %d, got %d", 1, config.AgentStream.Replicas)
	}

	if config.AgentStream.NoAck != false {
		t.Errorf("expected AgentStream.NoAck %v, got %v", false, config.AgentStream.NoAck)
	}

	if config.AgentStream.Duplicates != 5*time.Minute {
		t.Errorf("expected AgentStream.Duplicates %v, got %v", 5*time.Minute, config.AgentStream.Duplicates)
	}

	// Test agent subject prefix
	if config.AgentSubjectPrefix != defaultSubjectPrefix {
		t.Errorf("expected AgentSubjectPrefix %q, got %q", defaultSubjectPrefix, config.AgentSubjectPrefix)
	}
}

func TestConfig_Parse(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		expectedBucket string
		expectedStream string
		expectedPrefix string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "empty config - should use defaults",
			config:         Config{},
			expectedBucket: defaultAgentBucket,
			expectedStream: defaultAgentStream,
			expectedPrefix: defaultSubjectPrefix,
			wantErr:        false,
		},
		{
			name: "partial config - should fill defaults",
			config: Config{
				AgentBucket: client.BucketConfig{
					Bucket: "custom-bucket",
				},
			},
			expectedBucket: "custom-bucket",
			expectedStream: defaultAgentStream,
			expectedPrefix: defaultSubjectPrefix,
			wantErr:        false,
		},
		{
			name: "empty bucket name - should use default",
			config: Config{
				AgentBucket: client.BucketConfig{
					Bucket: "",
				},
			},
			expectedBucket: defaultAgentBucket,
			expectedStream: defaultAgentStream,
			expectedPrefix: defaultSubjectPrefix,
			wantErr:        false,
		},
		{
			name: "empty stream name - should use default",
			config: Config{
				AgentStream: client.StreamConfig{
					Name: "",
				},
			},
			expectedBucket: defaultAgentBucket,
			expectedStream: defaultAgentStream,
			expectedPrefix: defaultSubjectPrefix,
			wantErr:        false,
		},
		{
			name: "nil subjects - should use default",
			config: Config{
				AgentStream: client.StreamConfig{
					Subjects: nil,
				},
			},
			expectedBucket: defaultAgentBucket,
			expectedStream: defaultAgentStream,
			expectedPrefix: defaultSubjectPrefix,
			wantErr:        false,
		},
		{
			name: "empty subject prefix - should use default",
			config: Config{
				AgentSubjectPrefix: "",
			},
			expectedBucket: defaultAgentBucket,
			expectedStream: defaultAgentStream,
			expectedPrefix: defaultSubjectPrefix,
			wantErr:        false,
		},
		{
			name: "valid config - should keep values",
			config: Config{
				AgentBucket: client.BucketConfig{
					Bucket: "test-bucket",
				},
				AgentStream: client.StreamConfig{
					Name:     "test-stream",
					Subjects: []string{"test.>"},
				},
				AgentSubjectPrefix: "test.",
			},
			expectedBucket: "test-bucket",
			expectedStream: "test-stream",
			expectedPrefix: "test.",
			wantErr:        false,
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

			if config.AgentBucket.Bucket != tt.expectedBucket {
				t.Errorf("expected AgentBucket.Bucket %q, got %q", tt.expectedBucket, config.AgentBucket.Bucket)
			}

			if config.AgentStream.Name != tt.expectedStream {
				t.Errorf("expected AgentStream.Name %q, got %q", tt.expectedStream, config.AgentStream.Name)
			}

			if config.AgentSubjectPrefix != tt.expectedPrefix {
				t.Errorf("expected AgentSubjectPrefix %q, got %q", tt.expectedPrefix, config.AgentSubjectPrefix)
			}

			// Check that subjects are set correctly
			if config.AgentStream.Subjects == nil {
				t.Error("expected AgentStream.Subjects to be set")
			} else if len(config.AgentStream.Subjects) == 0 {
				t.Error("expected AgentStream.Subjects to have at least one subject")
			}
		})
	}
}

func TestConfig_Parse_EdgeCases(t *testing.T) {
	t.Run("whitespace bucket name", func(t *testing.T) {
		config := Config{
			AgentBucket: client.BucketConfig{
				Bucket: "   ",
			},
		}
		err := config.Parse()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Should use default bucket name since empty string after trim
		if config.AgentBucket.Bucket != defaultAgentBucket {
			t.Errorf("expected AgentBucket.Bucket %q, got %q", defaultAgentBucket, config.AgentBucket.Bucket)
		}
	})

	t.Run("whitespace stream name", func(t *testing.T) {
		config := Config{
			AgentStream: client.StreamConfig{
				Name: "   ",
			},
		}
		err := config.Parse()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Should use default stream name since empty string after trim
		if config.AgentStream.Name != defaultAgentStream {
			t.Errorf("expected AgentStream.Name %q, got %q", defaultAgentStream, config.AgentStream.Name)
		}
	})

	t.Run("whitespace subject prefix", func(t *testing.T) {
		config := Config{
			AgentSubjectPrefix: "   ",
		}
		err := config.Parse()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Should use default subject prefix since empty string after trim
		if config.AgentSubjectPrefix != defaultSubjectPrefix {
			t.Errorf("expected AgentSubjectPrefix %q, got %q", defaultSubjectPrefix, config.AgentSubjectPrefix)
		}
	})

	t.Run("invalid bucket name", func(t *testing.T) {
		longName := "very-long-bucket-name-that-exceeds-normal-limits-and-should-fail-validation"
		config := Config{
			AgentBucket: client.BucketConfig{
				Bucket: longName,
			},
		}
		err := config.Parse()
		if err == nil {
			t.Error("expected error for invalid bucket name, got nil")
		}
		if !strings.Contains(err.Error(), "invalid agent bucket name") {
			t.Errorf("expected bucket validation error, got: %v", err)
		}
	})

	t.Run("very long stream name", func(t *testing.T) {
		longName := "very-long-stream-name-that-exceeds-normal-limits-and-should-still-work"
		config := Config{
			AgentStream: client.StreamConfig{
				Name: longName,
			},
		}
		err := config.Parse()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if config.AgentStream.Name != longName {
			t.Errorf("expected AgentStream.Name %q, got %q", longName, config.AgentStream.Name)
		}
	})
}

func TestConfig_Constants(t *testing.T) {
	// Test that constants are reasonable values
	if defaultAgentBucket == "" {
		t.Error("defaultAgentBucket should not be empty")
	}

	if defaultAgentStream == "" {
		t.Error("defaultAgentStream should not be empty")
	}

	if defaultSubjectPrefix == "" {
		t.Error("defaultSubjectPrefix should not be empty")
	}

	// Test that subject prefix ends with "."
	if defaultSubjectPrefix[len(defaultSubjectPrefix)-1] != '.' {
		t.Error("defaultSubjectPrefix should end with '.'")
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	config := DefaultConfig()

	// Test that default values are reasonable
	if config.AgentBucket.History <= 0 {
		t.Error("AgentBucket.History should be positive")
	}

	if config.AgentBucket.TTL <= 0 {
		t.Error("AgentBucket.TTL should be positive")
	}

	if config.AgentBucket.Replicas <= 0 {
		t.Error("AgentBucket.Replicas should be positive")
	}

	if config.AgentStream.MaxAge <= 0 {
		t.Error("AgentStream.MaxAge should be positive")
	}

	if config.AgentStream.MaxBytes <= 0 {
		t.Error("AgentStream.MaxBytes should be positive")
	}

	if config.AgentStream.Duplicates <= 0 {
		t.Error("AgentStream.Duplicates should be positive")
	}

	// Test that retention policy is valid
	if config.AgentStream.Retention != jetstream.LimitsPolicy {
		t.Error("AgentStream.Retention should be LimitsPolicy")
	}

	// Test that storage type is valid
	if config.AgentBucket.Storage != jetstream.FileStorage {
		t.Error("AgentBucket.Storage should be FileStorage")
	}

	if config.AgentStream.Storage != jetstream.FileStorage {
		t.Error("AgentStream.Storage should be FileStorage")
	}
}

// Benchmark tests
func BenchmarkConfig_Parse(b *testing.B) {
	config := Config{
		AgentBucket: client.BucketConfig{
			Bucket: "test-bucket",
		},
		AgentStream: client.StreamConfig{
			Name:     "test-stream",
			Subjects: []string{"test.>"},
		},
		AgentSubjectPrefix: "test.",
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
