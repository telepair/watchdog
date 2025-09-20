package agent

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.ID != defaultID {
		t.Errorf("expected ID %q, got %q", defaultID, config.ID)
	}

	if config.ReportInterval != defaultReportInterval {
		t.Errorf("expected ReportInterval %d, got %d", defaultReportInterval, config.ReportInterval)
	}

	if config.HeartbeatInterval != defaultHeartbeatInterval {
		t.Errorf("expected HeartbeatInterval %d, got %d", defaultHeartbeatInterval, config.HeartbeatInterval)
	}
}

func TestConfig_Parse(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		expectedID     string
		expectedReport int
		expectedHeart  int
	}{
		{
			name:           "empty config - should use defaults",
			config:         Config{},
			expectedID:     defaultID,
			expectedReport: defaultReportInterval,
			expectedHeart:  defaultHeartbeatInterval,
		},
		{
			name: "partial config - should fill defaults",
			config: Config{
				ID: "custom-agent",
			},
			expectedID:     "custom-agent",
			expectedReport: defaultReportInterval,
			expectedHeart:  defaultHeartbeatInterval,
		},
		{
			name: "zero values - should use defaults",
			config: Config{
				ID:                "",
				ReportInterval:    0,
				HeartbeatInterval: 0,
			},
			expectedID:     defaultID,
			expectedReport: defaultReportInterval,
			expectedHeart:  defaultHeartbeatInterval,
		},
		{
			name: "negative values - should use defaults",
			config: Config{
				ID:                "test-agent",
				ReportInterval:    -1,
				HeartbeatInterval: -5,
			},
			expectedID:     "test-agent",
			expectedReport: defaultReportInterval,
			expectedHeart:  defaultHeartbeatInterval,
		},
		{
			name: "valid config - should keep values",
			config: Config{
				ID:                "valid-agent",
				ReportInterval:    300,
				HeartbeatInterval: 10,
			},
			expectedID:     "valid-agent",
			expectedReport: 300,
			expectedHeart:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.config
			err := config.Parse()

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if config.ID != tt.expectedID {
				t.Errorf("expected ID %q, got %q", tt.expectedID, config.ID)
			}

			if config.ReportInterval != tt.expectedReport {
				t.Errorf("expected ReportInterval %d, got %d", tt.expectedReport, config.ReportInterval)
			}

			if config.HeartbeatInterval != tt.expectedHeart {
				t.Errorf("expected HeartbeatInterval %d, got %d", tt.expectedHeart, config.HeartbeatInterval)
			}
		})
	}
}

func TestConfig_Parse_EdgeCases(t *testing.T) {
	t.Run("whitespace ID", func(t *testing.T) {
		config := Config{
			ID: "   ",
		}
		err := config.Parse()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Should use default ID since empty string after trim
		if config.ID != defaultID {
			t.Errorf("expected ID %q, got %q", defaultID, config.ID)
		}
	})

	t.Run("very large intervals", func(t *testing.T) {
		config := Config{
			ID:                "large-interval-agent",
			ReportInterval:    999999,
			HeartbeatInterval: 999999,
		}
		err := config.Parse()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Should keep large values
		if config.ReportInterval != 999999 {
			t.Errorf("expected ReportInterval %d, got %d", 999999, config.ReportInterval)
		}
		if config.HeartbeatInterval != 999999 {
			t.Errorf("expected HeartbeatInterval %d, got %d", 999999, config.HeartbeatInterval)
		}
	})
}

func TestConfig_Parse_InvalidAgentID(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		expectError bool
		errorMatch  string
	}{
		{
			name:        "invalid characters - dots",
			id:          "agent.with.dots",
			expectError: true,
			errorMatch:  "invalid characters",
		},
		{
			name:        "invalid characters - spaces",
			id:          "agent with spaces",
			expectError: true,
			errorMatch:  "invalid characters",
		},
		{
			name:        "invalid characters - special chars",
			id:          "agent@#$%",
			expectError: true,
			errorMatch:  "invalid characters",
		},
		{
			name:        "starts with hyphen",
			id:          "-agent",
			expectError: true,
			errorMatch:  "cannot start or end with hyphen",
		},
		{
			name:        "ends with hyphen",
			id:          "agent-",
			expectError: true,
			errorMatch:  "cannot start or end with hyphen",
		},
		{
			name:        "consecutive hyphens",
			id:          "agent--test",
			expectError: true,
			errorMatch:  "consecutive hyphens",
		},
		{
			name:        "too long",
			id:          strings.Repeat("a", 64),
			expectError: true,
			errorMatch:  "too long",
		},
		{
			name:        "valid alphanumeric",
			id:          "agent123",
			expectError: false,
		},
		{
			name:        "valid with hyphens",
			id:          "agent-test-123",
			expectError: false,
		},
		{
			name:        "valid with underscores",
			id:          "agent_test_123",
			expectError: false,
		},
		{
			name:        "valid mixed",
			id:          "agent-test_123",
			expectError: false,
		},
		{
			name:        "single character",
			id:          "a",
			expectError: false,
		},
		{
			name:        "max length",
			id:          strings.Repeat("a", 63),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				ID:                tt.id,
				ReportInterval:    600,
				HeartbeatInterval: 5,
			}
			err := config.Parse()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for ID %q, but got none", tt.id)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMatch) {
					t.Errorf("expected error containing %q, got %q", tt.errorMatch, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for valid ID %q: %v", tt.id, err)
				}
			}
		})
	}
}

func TestValidateAgentID(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		wantError bool
	}{
		{"empty", "", true},
		{"valid simple", "agent", false},
		{"valid with numbers", "agent123", false},
		{"valid with hyphens", "agent-test", false},
		{"valid with underscores", "agent_test", false},
		{"invalid dots", "agent.test", true},
		{"invalid spaces", "agent test", true},
		{"starts with hyphen", "-agent", true},
		{"ends with hyphen", "agent-", true},
		{"consecutive hyphens", "agent--test", true},
		{"too long", strings.Repeat("a", 64), true},
		{"max valid length", strings.Repeat("a", 63), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgentID(tt.id)
			if (err != nil) != tt.wantError {
				t.Errorf("validateAgentID(%q) error = %v, wantError %v", tt.id, err, tt.wantError)
			}
		})
	}
}

func TestConfig_Constants(t *testing.T) {
	// Test that constants are reasonable values
	if defaultID == "" {
		t.Error("defaultID should not be empty")
	}

	if defaultReportInterval <= 0 {
		t.Errorf("defaultReportInterval should be positive, got %d", defaultReportInterval)
	}

	if defaultHeartbeatInterval <= 0 {
		t.Errorf("defaultHeartbeatInterval should be positive, got %d", defaultHeartbeatInterval)
	}

	// Report interval should be larger than heartbeat interval
	if defaultReportInterval <= defaultHeartbeatInterval {
		t.Errorf("defaultReportInterval (%d) should be larger than defaultHeartbeatInterval (%d)",
			defaultReportInterval, defaultHeartbeatInterval)
	}
}

// Benchmark tests
func BenchmarkConfig_Parse(b *testing.B) {
	config := Config{
		ID:                "benchmark-agent",
		ReportInterval:    600,
		HeartbeatInterval: 5,
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
