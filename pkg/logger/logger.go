package logger

import (
	"context"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// New creates a new slog.Logger with the given configuration.
func New(config Config) (*slog.Logger, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	var handlers []slog.Handler

	// Console handler
	if config.Console.Enabled {
		consoleLevel, _ := config.Console.Level.toSlogLevel()

		var consoleHandler slog.Handler
		switch config.Console.Format {
		case FormatJSON:
			consoleHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level: consoleLevel,
			})
		case FormatText:
			consoleHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: consoleLevel,
			})
		}
		handlers = append(handlers, consoleHandler)
	}

	// File handler
	if config.File.Enabled {
		fileLevel, _ := config.File.Level.toSlogLevel()

		fileWriter := &lumberjack.Logger{
			Filename:   config.File.Filename,
			MaxSize:    config.File.MaxSize,
			MaxBackups: config.File.MaxBackups,
			MaxAge:     config.File.MaxAge,
			Compress:   config.File.Compress,
		}

		var fileHandler slog.Handler
		switch config.File.Format {
		case FormatJSON:
			fileHandler = slog.NewJSONHandler(fileWriter, &slog.HandlerOptions{
				Level: fileLevel,
			})
		case FormatText:
			fileHandler = slog.NewTextHandler(fileWriter, &slog.HandlerOptions{
				Level: fileLevel,
			})
		}
		handlers = append(handlers, fileHandler)
	}

	// Create multi-handler
	var handler slog.Handler
	switch len(handlers) {
	case 0:
		// Fallback to default console handler
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	case 1:
		handler = handlers[0]
	default:
		handler = NewMultiHandler(handlers...)
	}

	return slog.New(handler), nil
}

// SetDefault sets the default logger with the given configuration.
func SetDefault(config Config) error {
	logger, err := New(config)
	if err != nil {
		return err
	}

	slog.SetDefault(logger)
	return nil
}

// ComponentLogger returns a logger with the given component name as a group.
func ComponentLogger(component string) *slog.Logger {
	return slog.Default().With("component", component)
}

// MultiHandler handles multiple slog handlers.
type MultiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler creates a new multi-handler.
func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

// Enabled reports whether the handler handles records at the given level.
func (m *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle handles the record.
func (m *MultiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, record.Level) {
			if err := h.Handle(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

// WithAttrs returns a new handler with the given attributes.
func (m *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return NewMultiHandler(handlers...)
}

// WithGroup returns a new handler with the given group name.
func (m *MultiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return NewMultiHandler(handlers...)
}
