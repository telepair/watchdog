package logger

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "default config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "console only",
			config: Config{
				Console: ConsoleConfig{
					Enabled: true,
					Level:   LevelInfo,
					Format:  FormatText,
				},
				File: FileConfig{
					Enabled: false,
				},
			},
			wantErr: false,
		},
		{
			name: "file only",
			config: Config{
				Console: ConsoleConfig{
					Enabled: false,
				},
				File: FileConfig{
					Enabled:    true,
					Level:      LevelInfo,
					Format:     FormatJSON,
					Filename:   filepath.Join(t.TempDir(), "test.log"),
					MaxSize:    10,
					MaxBackups: 1,
					MaxAge:     1,
					Compress:   false,
				},
			},
			wantErr: false,
		},
		{
			name: "both console and file",
			config: Config{
				Console: ConsoleConfig{
					Enabled: true,
					Level:   LevelDebug,
					Format:  FormatJSON,
				},
				File: FileConfig{
					Enabled:    true,
					Level:      LevelWarn,
					Format:     FormatText,
					Filename:   filepath.Join(t.TempDir(), "test2.log"),
					MaxSize:    10,
					MaxBackups: 1,
					MaxAge:     1,
					Compress:   false,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid config",
			config: Config{
				Console: ConsoleConfig{
					Enabled: true,
					Level:   Level("invalid"),
					Format:  FormatText,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && logger == nil {
				t.Error("New() returned nil logger without error")
			}
		})
	}
}

func TestSetDefault(t *testing.T) {
	// Save original default logger
	originalDefault := slog.Default()

	config := Config{
		Console: ConsoleConfig{
			Enabled: true,
			Level:   LevelDebug,
			Format:  FormatText,
		},
		File: FileConfig{
			Enabled: false,
		},
	}

	err := SetDefault(config)
	if err != nil {
		t.Fatalf("SetDefault() error = %v", err)
	}

	// Test that default logger was changed
	if slog.Default() == originalDefault {
		t.Error("SetDefault() did not change the default logger")
	}

	// Test with invalid config
	invalidConfig := Config{
		Console: ConsoleConfig{
			Enabled: true,
			Level:   Level("invalid"),
			Format:  FormatText,
		},
	}

	err = SetDefault(invalidConfig)
	if err == nil {
		t.Error("SetDefault() should have returned error for invalid config")
	}

	// Restore original default logger
	slog.SetDefault(originalDefault)
}

func TestComponentLogger(t *testing.T) {
	// Set up a test logger with a buffer to capture output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	testLogger := slog.New(handler)
	slog.SetDefault(testLogger)

	componentLogger := ComponentLogger("test-component")
	componentLogger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "component=test-component") {
		t.Errorf("expected output to contain component=test-component, got: %s", output)
	}

	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got: %s", output)
	}
}

func TestMultiHandler(t *testing.T) {
	var buf1, buf2 bytes.Buffer

	handler1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	handler2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})

	multiHandler := NewMultiHandler(handler1, handler2)
	logger := slog.New(multiHandler)

	ctx := context.Background()

	// Test Info level - should only go to handler1
	logger.InfoContext(ctx, "info message")

	if !strings.Contains(buf1.String(), "info message") {
		t.Error("handler1 should have received info message")
	}

	if strings.Contains(buf2.String(), "info message") {
		t.Error("handler2 should not have received info message")
	}

	// Clear buffers
	buf1.Reset()
	buf2.Reset()

	// Test Warn level - should go to both handlers
	logger.WarnContext(ctx, "warn message")

	if !strings.Contains(buf1.String(), "warn message") {
		t.Error("handler1 should have received warn message")
	}

	if !strings.Contains(buf2.String(), "warn message") {
		t.Error("handler2 should have received warn message")
	}
}

func TestMultiHandler_Enabled(t *testing.T) {
	var buf1, buf2 bytes.Buffer

	handler1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	handler2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})

	multiHandler := NewMultiHandler(handler1, handler2)
	ctx := context.Background()

	tests := []struct {
		name     string
		level    slog.Level
		expected bool
	}{
		{
			name:     "debug level",
			level:    slog.LevelDebug,
			expected: false,
		},
		{
			name:     "info level",
			level:    slog.LevelInfo,
			expected: true,
		},
		{
			name:     "warn level",
			level:    slog.LevelWarn,
			expected: true,
		},
		{
			name:     "error level",
			level:    slog.LevelError,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled := multiHandler.Enabled(ctx, tt.level)
			if enabled != tt.expected {
				t.Errorf("Enabled() = %v, want %v", enabled, tt.expected)
			}
		})
	}
}

func TestMultiHandler_WithAttrs(t *testing.T) {
	var buf1, buf2 bytes.Buffer

	handler1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	handler2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	multiHandler := NewMultiHandler(handler1, handler2)
	attrs := []slog.Attr{
		slog.String("key1", "value1"),
		slog.String("key2", "value2"),
	}

	newHandler := multiHandler.WithAttrs(attrs)
	logger := slog.New(newHandler)

	logger.Info("test message")

	output1 := buf1.String()
	output2 := buf2.String()

	if !strings.Contains(output1, "key1=value1") {
		t.Errorf("handler1 output should contain attributes, got: %s", output1)
	}

	if !strings.Contains(output2, "key1=value1") {
		t.Errorf("handler2 output should contain attributes, got: %s", output2)
	}
}

func TestMultiHandler_WithGroup(t *testing.T) {
	var buf1, buf2 bytes.Buffer

	handler1 := slog.NewJSONHandler(&buf1, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	handler2 := slog.NewJSONHandler(&buf2, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	multiHandler := NewMultiHandler(handler1, handler2)
	newHandler := multiHandler.WithGroup("test-group")
	logger := slog.New(newHandler)

	logger.Info("test message", "key", "value")

	output1 := buf1.String()
	output2 := buf2.String()

	if !strings.Contains(output1, "test-group") {
		t.Errorf("handler1 output should contain group, got: %s", output1)
	}

	if !strings.Contains(output2, "test-group") {
		t.Errorf("handler2 output should contain group, got: %s", output2)
	}
}

func TestNew_EmptyHandlers(t *testing.T) {
	// Test case where no handlers are enabled - should fall back to default
	config := Config{
		Console: ConsoleConfig{
			Enabled: false,
		},
		File: FileConfig{
			Enabled: false,
		},
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if logger == nil {
		t.Error("New() returned nil logger")
	}

	// Verify logger works
	logger.Info("test message")
}
