package logger

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// Format represents log format type.
type Format string

// Level represents log level type.
type Level string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

const (
	defaultMaxSize    = 100 // 100MB
	defaultMaxBackups = 3
	defaultMaxAge     = 7 // 7 days
	defaultFilename   = "watchdog.log"
)

// ConsoleConfig represents console output configuration.
type ConsoleConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Level   Level  `yaml:"level"   json:"level"`
	Format  Format `yaml:"format"  json:"format"`
}

// FileConfig represents file output configuration.
type FileConfig struct {
	Enabled    bool   `yaml:"enabled"     json:"enabled"`
	Level      Level  `yaml:"level"       json:"level"`
	Format     Format `yaml:"format"      json:"format"`
	Filename   string `yaml:"filename"    json:"filename"`
	MaxSize    int    `yaml:"max_size"    json:"max_size"`    // MB
	MaxBackups int    `yaml:"max_backups" json:"max_backups"` // number of backups
	MaxAge     int    `yaml:"max_age"     json:"max_age"`     // days
	Compress   bool   `yaml:"compress"    json:"compress"`
}

// Config represents logger configuration.
type Config struct {
	Console ConsoleConfig `yaml:"console" json:"console"`
	File    FileConfig    `yaml:"file"    json:"file"`
}

// DefaultConfig returns default logger configuration.
func DefaultConfig() Config {
	return Config{
		Console: ConsoleConfig{
			Enabled: true,
			Level:   LevelInfo,
			Format:  FormatText,
		},
		File: FileConfig{
			Enabled:    false,
			Level:      LevelInfo,
			Format:     FormatJSON,
			Filename:   defaultFilename,
			MaxSize:    defaultMaxSize, // 100MB
			MaxBackups: defaultMaxBackups,
			MaxAge:     defaultMaxAge, // 7 days
			Compress:   true,
		},
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Console.Enabled {
		if _, err := c.Console.Level.toSlogLevel(); err != nil {
			return fmt.Errorf("invalid console log level: %w", err)
		}
		if c.Console.Format != FormatText && c.Console.Format != FormatJSON {
			return fmt.Errorf("invalid console format: %s", c.Console.Format)
		}
	}

	return c.File.Validate()
}

// Validate validates the file configuration.
func (fc *FileConfig) Validate() error {
	if !fc.Enabled {
		return nil
	}
	if _, err := fc.Level.toSlogLevel(); err != nil {
		return fmt.Errorf("invalid file log level: %w", err)
	}
	if fc.Format != FormatText && fc.Format != FormatJSON {
		return fmt.Errorf("invalid file format: %s", fc.Format)
	}
	if fc.Filename == "" {
		return errors.New("file filename cannot be empty")
	}
	if fc.MaxSize <= 0 {
		return errors.New("file max_size must be positive")
	}
	if fc.MaxBackups < 0 {
		return errors.New("file max_backups cannot be negative")
	}
	if fc.MaxAge < 0 {
		return errors.New("file max_age cannot be negative")
	}
	return nil
}

// toSlogLevel converts Level to slog.Level.
func (l Level) toSlogLevel() (slog.Level, error) {
	switch strings.ToLower(string(l)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level: %s", l)
	}
}
