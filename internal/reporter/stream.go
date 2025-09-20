package reporter

import (
	"context"
	"errors"
	"fmt"

	"github.com/telepair/watchdog/pkg/natsx/client"
)

type Stream struct {
	stream *client.Stream
}

func GetStream(streamName string, natsClient *client.Client) (*Stream, error) {
	if natsClient == nil {
		return nil, errors.New("NATS client is required")
	}

	stream, err := natsClient.GetStream(streamName)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent stream: %w", err)
	}
	return &Stream{stream: stream}, nil
}

func (s *Stream) Publish(ctx context.Context, subject string, data any) error {
	if err := client.ValidateSubject(subject); err != nil {
		return fmt.Errorf("invalid subject: %w", err)
	}

	var payload []byte
	switch data := data.(type) {
	case []byte:
		payload = data
	case string:
		payload = []byte(data)
	}
	return s.stream.Publish(ctx, subject, payload)
}
