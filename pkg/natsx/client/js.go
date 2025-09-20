package client

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
)

type StreamConfig = jetstream.StreamConfig

type Stream struct {
	name   string
	js     jetstream.JetStream
	stream jetstream.Stream
	logger *slog.Logger
}

func (c *Client) EnsureStream(ctx context.Context, config StreamConfig) (*Stream, error) {
	if config.Name == "" {
		return nil, fmt.Errorf("stream name cannot be empty")
	}

	// Try to get existing stream first
	stream, err := c.js.Stream(ctx, config.Name)
	if err == nil {
		// Stream exists, return it
		return &Stream{
			name:   config.Name,
			js:     c.js,
			stream: stream,
			logger: c.logger.With("stream", config.Name),
		}, nil
	}

	// Stream doesn't exist, create it
	stream, err = c.js.CreateStream(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	return &Stream{
		name:   config.Name,
		js:     c.js,
		stream: stream,
		logger: c.logger.With("stream", config.Name),
	}, nil
}

func (c *Client) CreateStream(ctx context.Context, config StreamConfig) (*Stream, error) {
	if config.Name == "" {
		return nil, fmt.Errorf("stream name cannot be empty")
	}
	stream, err := c.js.CreateStream(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}
	return &Stream{
		name:   config.Name,
		js:     c.js,
		stream: stream,
		logger: c.logger.With("stream", config.Name),
	}, nil
}

func (c *Client) GetStream(name string) (*Stream, error) {
	stream, err := c.js.Stream(context.Background(), name)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream: %w", err)
	}
	return &Stream{
		name:   name,
		js:     c.js,
		stream: stream,
		logger: c.logger.With("stream", name),
	}, nil
}

func (s *Stream) Publish(ctx context.Context, subject string, data []byte) error {
	if err := ValidateSubject(subject); err != nil {
		return fmt.Errorf("invalid subject: %w", err)
	}

	_, err := s.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	return nil
}
