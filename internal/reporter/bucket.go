package reporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/telepair/watchdog/pkg/natsx/client"
)

type Bucket struct {
	bucket *client.Bucket
}

func GetBucket(bucketName string, natsClient *client.Client) (*Bucket, error) {
	if natsClient == nil {
		return nil, errors.New("NATS client is required")
	}

	bucket, err := natsClient.GetBucket(bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent bucket: %w", err)
	}

	return &Bucket{bucket: bucket}, nil
}

func (a *Bucket) Put(ctx context.Context, key string, data any) error {
	var payload []byte
	switch data := data.(type) {
	case []byte:
		payload = data
	case string:
		payload = []byte(data)
	default:
		var err error
		payload, err = json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
	}
	return a.bucket.Put(ctx, key, payload)
}
