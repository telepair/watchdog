package logger

import (
	"log/slog"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test console config
	if !config.Console.Enabled {
		t.Error("expected console to be enabled by default")
	}

	if config.Console.Level != LevelInfo {
		t.Errorf("expected console level to be %s, got %s", LevelInfo, config.Console.Level)
	}

	if config.Console.Format != FormatText {
		t.Errorf("expected console format to be %s, got %s", FormatText, config.Console.Format)
	}

	// Test file config
	if config.File.Enabled {
		t.Error("expected file to be disabled by default")
	}

	if config.File.Level != LevelInfo {
		t.Errorf("expected file level to be %s, got %s", LevelInfo, config.File.Level)
	}

	if config.File.Format != FormatJSON {
		t.Errorf("expected file format to be %s, got %s", FormatJSON, config.File.Format)
	}

	if config.File.Filename != defaultFilename {
		t.Errorf("expected file filename to be %s, got %s", defaultFilename, config.File.Filename)
	}

	if config.File.MaxSize != defaultMaxSize {
		t.Errorf("expected file max size to be %d, got %d", defaultMaxSize, config.File.MaxSize)
	}

	if config.File.MaxBackups != defaultMaxBackups {
		t.Errorf("expected file max backups to be %d, got %d", defaultMaxBackups, config.File.MaxBackups)
	}

	if config.File.MaxAge != defaultMaxAge {
		t.Errorf("expected file max age to be %d, got %d", defaultMaxAge, config.File.MaxAge)
	}

	if !config.File.Compress {
		t.Error("expected file compress to be true by default")
	}
}

func TestConfig_Validate(t *testing.T) {
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
			name: "valid config with console only",
			config: Config{
				Console: ConsoleConfig{
					Enabled: true,
					Level:   LevelDebug,
					Format:  FormatJSON,
				},
				File: FileConfig{
					Enabled: false,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid console level",
			config: Config{
				Console: ConsoleConfig{
					Enabled: true,
					Level:   Level("invalid"),
					Format:  FormatText,
				},
				File: FileConfig{
					Enabled: false,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid console format",
			config: Config{
				Console: ConsoleConfig{
					Enabled: true,
					Level:   LevelInfo,
					Format:  Format("invalid"),
				},
				File: FileConfig{
					Enabled: false,
				},
			},
			wantErr: true,
		},
		{
			name: "valid file config",
			config: Config{
				Console: ConsoleConfig{
					Enabled: false,
				},
				File: FileConfig{
					Enabled:    true,
					Level:      LevelWarn,
					Format:     FormatJSON,
					Filename:   "test.log",
					MaxSize:    50,
					MaxBackups: 5,
					MaxAge:     10,
					Compress:   true,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFileConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  FileConfig
		wantErr bool
	}{
		{
			name: "disabled file config",
			config: FileConfig{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "valid file config",
			config: FileConfig{
				Enabled:    true,
				Level:      LevelInfo,
				Format:     FormatJSON,
				Filename:   "test.log",
				MaxSize:    100,
				MaxBackups: 3,
				MaxAge:     7,
				Compress:   true,
			},
			wantErr: false,
		},
		{
			name: "invalid level",
			config: FileConfig{
				Enabled:  true,
				Level:    Level("invalid"),
				Format:   FormatJSON,
				Filename: "test.log",
				MaxSize:  100,
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			config: FileConfig{
				Enabled:  true,
				Level:    LevelInfo,
				Format:   Format("invalid"),
				Filename: "test.log",
				MaxSize:  100,
			},
			wantErr: true,
		},
		{
			name: "empty filename",
			config: FileConfig{
				Enabled: true,
				Level:   LevelInfo,
				Format:  FormatJSON,
				MaxSize: 100,
			},
			wantErr: true,
		},
		{
			name: "invalid max size",
			config: FileConfig{
				Enabled:  true,
				Level:    LevelInfo,
				Format:   FormatJSON,
				Filename: "test.log",
				MaxSize:  0,
			},
			wantErr: true,
		},
		{
			name: "negative max backups",
			config: FileConfig{
				Enabled:    true,
				Level:      LevelInfo,
				Format:     FormatJSON,
				Filename:   "test.log",
				MaxSize:    100,
				MaxBackups: -1,
			},
			wantErr: true,
		},
		{
			name: "negative max age",
			config: FileConfig{
				Enabled:  true,
				Level:    LevelInfo,
				Format:   FormatJSON,
				Filename: "test.log",
				MaxSize:  100,
				MaxAge:   -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("FileConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLevel_toSlogLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		expected slog.Level
		wantErr  bool
	}{
		{
			name:     "debug level",
			level:    LevelDebug,
			expected: slog.LevelDebug,
			wantErr:  false,
		},
		{
			name:     "info level",
			level:    LevelInfo,
			expected: slog.LevelInfo,
			wantErr:  false,
		},
		{
			name:     "warn level",
			level:    LevelWarn,
			expected: slog.LevelWarn,
			wantErr:  false,
		},
		{
			name:     "error level",
			level:    LevelError,
			expected: slog.LevelError,
			wantErr:  false,
		},
		{
			name:     "uppercase debug",
			level:    Level("DEBUG"),
			expected: slog.LevelDebug,
			wantErr:  false,
		},
		{
			name:     "mixed case info",
			level:    Level("Info"),
			expected: slog.LevelInfo,
			wantErr:  false,
		},
		{
			name:    "invalid level",
			level:   Level("invalid"),
			wantErr: true,
		},
		{
			name:    "empty level",
			level:   Level(""),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.level.ToSlogLevel()
			if (err != nil) != tt.wantErr {
				t.Errorf("Level.toSlogLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("Level.toSlogLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}
