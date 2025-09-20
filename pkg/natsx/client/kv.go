package client

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
)

type BucketConfig = jetstream.KeyValueConfig

// Bucket manages NATS key-value stores.
type Bucket struct {
	name   string
	js     jetstream.JetStream
	kv     jetstream.KeyValue
	logger *slog.Logger
}

func (c *Client) EnsureBucket(ctx context.Context, config BucketConfig) (*Bucket, error) {
	if err := ValidateBucketName(config.Bucket); err != nil {
		return nil, fmt.Errorf("invalid bucket name: %w", err)
	}

	// Try to get existing bucket first
	kv, err := c.js.KeyValue(ctx, config.Bucket)
	if err != nil {
		// If bucket doesn't exist, create it
		kv, err = c.js.CreateKeyValue(ctx, config)
		if err != nil {
			c.logger.Error("failed to ensure KV bucket", "error", err)
			return nil, fmt.Errorf("failed to ensure KV bucket: %w", err)
		}
	}

	return &Bucket{
		name:   config.Bucket,
		js:     c.js,
		kv:     kv,
		logger: c.logger.With("bucket", config.Bucket),
	}, nil
}

func (c *Client) CreateBucket(ctx context.Context, config BucketConfig) (*Bucket, error) {
	if err := ValidateBucketName(config.Bucket); err != nil {
		return nil, fmt.Errorf("invalid bucket name: %w", err)
	}

	kv, err := c.js.CreateKeyValue(ctx, config)
	if err != nil {
		// Try to get existing bucket
		kv, err = c.js.KeyValue(ctx, config.Bucket)
		if err != nil {
			c.logger.Error("failed to create or get KV bucket", "error", err)
			return nil, fmt.Errorf("failed to create or get KV bucket: %w", err)
		}
	}

	return &Bucket{
		name:   config.Bucket,
		js:     c.js,
		kv:     kv,
		logger: c.logger.With("bucket", config.Bucket),
	}, nil
}

func (c *Client) GetBucket(name string) (*Bucket, error) {
	if err := ValidateBucketName(name); err != nil {
		return nil, fmt.Errorf("invalid bucket name: %w", err)
	}

	kv, err := c.js.KeyValue(context.Background(), name)
	if err != nil {
		return nil, fmt.Errorf("failed to get KV bucket: %w", err)
	}
	return &Bucket{name: name, js: c.js, kv: kv, logger: c.logger.With("bucket", name)}, nil
}

// Put stores a value with the given key.
func (b *Bucket) Put(ctx context.Context, key string, value []byte) error {
	if err := ValidateKey(key); err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}
	if err := ValidateValue(value); err != nil {
		return fmt.Errorf("invalid value: %w", err)
	}

	_, err := b.kv.Put(ctx, key, value)
	if err != nil {
		b.logger.ErrorContext(ctx, "failed to put key-value pair", "error", err)
		return err
	}
	b.logger.DebugContext(ctx, "put key-value pair", "key", key)
	return nil
}

// Get retrieves a value by key.
func (b *Bucket) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ValidateKey(key); err != nil {
		return nil, fmt.Errorf("invalid key: %w", err)
	}

	entry, err := b.kv.Get(ctx, key)
	if err != nil {
		b.logger.ErrorContext(ctx, "failed to get key-value pair", "error", err)
		return nil, err
	}
	b.logger.DebugContext(ctx, "got key-value pair", "key", key)
	return entry.Value(), nil
}

// Delete removes a key-value pair.
func (b *Bucket) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}

	if err := b.kv.Delete(ctx, key); err != nil {
		b.logger.ErrorContext(ctx, "failed to delete key-value pair", "error", err)
		return err
	}
	b.logger.DebugContext(ctx, "deleted key-value pair", "key", key)
	return nil
}
