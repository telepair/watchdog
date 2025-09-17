package client

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const defaultInitTimeout = 5 * time.Second

// BucketConfig is the configuration for a NATS key-value store bucket.
type BucketConfig struct {
	Name         string        `yaml:"name"           json:"name"`
	History      uint8         `yaml:"history"        json:"history"`
	Replicas     int           `yaml:"replicas"       json:"replicas"`
	OnMemory     bool          `yaml:"on_memory"      json:"on_memory"`
	Compression  bool          `yaml:"compression"    json:"compression"`
	MaxBytes     int64         `yaml:"max_bytes"      json:"max_bytes"`
	MaxValueSize int32         `yaml:"max_value_size" json:"max_value_size"`
	Tags         []string      `yaml:"tags"           json:"tags"`
	Cluster      string        `yaml:"cluster"        json:"cluster"`
	TTL          time.Duration `yaml:"ttl"            json:"ttl"`
}

// Validate validates the bucket config.
func (c *BucketConfig) Validate() error {
	if err := ValidateBucketName(c.Name); err != nil {
		return fmt.Errorf("invalid bucket name: %w", err)
	}
	return nil
}

// KVManager manages NATS key-value stores.
type KVManager struct {
	js     jetstream.JetStream
	kv     jetstream.KeyValue
	config *BucketConfig
	logger *slog.Logger
}

// NewKVManager creates a new KV manager.
func NewKVManager(js jetstream.JetStream, config BucketConfig) (*KVManager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid bucket config: %w", err)
	}

	// Create a copy of the config to avoid external mutations
	configCopy := config
	manager := &KVManager{
		js:     js,
		config: &configCopy,
	}
	manager.logger = slog.Default().With("component", fmt.Sprintf("natsx.kv.%s", config.Name))

	if err := manager.initKV(); err != nil {
		return nil, fmt.Errorf("failed to initialize KV store: %w", err)
	}

	return manager, nil
}

// initKV initializes the key-value store.
func (m *KVManager) initKV() error {
	cfg := jetstream.KeyValueConfig{
		Bucket:       m.config.Name,
		TTL:          m.config.TTL,
		History:      m.config.History,
		Replicas:     m.config.Replicas,
		Compression:  m.config.Compression,
		MaxBytes:     m.config.MaxBytes,
		MaxValueSize: m.config.MaxValueSize,
	}
	if m.config.OnMemory {
		cfg.Storage = jetstream.MemoryStorage
	} else {
		cfg.Storage = jetstream.FileStorage
	}

	if m.config.Cluster != "" {
		cfg.Placement = &jetstream.Placement{
			Cluster: m.config.Cluster,
		}
	}
	if len(m.config.Tags) > 0 {
		if cfg.Placement == nil {
			cfg.Placement = &jetstream.Placement{}
		}
		cfg.Placement.Tags = m.config.Tags
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultInitTimeout)
	defer cancel()
	kv, err := m.js.CreateKeyValue(ctx, cfg)
	if err != nil {
		// Try to get existing bucket
		kv, err = m.js.KeyValue(ctx, m.config.Name)
		if err != nil {
			m.logger.Error("failed to create or get KV bucket", "error", err)
			return fmt.Errorf("failed to create or get KV bucket: %w", err)
		}
	}

	m.kv = kv
	m.logger.Info("KV bucket initialized", "bucket", m.config.Name)
	return nil
}

// KeyValue returns the underlying key-value store.
func (m *KVManager) KeyValue() jetstream.KeyValue {
	return m.kv
}

// Put stores a value with the given key.
func (m *KVManager) Put(ctx context.Context, key string, value []byte) error {
	if err := ValidateKey(key); err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}
	if err := ValidateValue(value); err != nil {
		return fmt.Errorf("invalid value: %w", err)
	}

	_, err := m.kv.Put(ctx, key, value)
	if err != nil {
		m.logger.ErrorContext(ctx, "failed to put key-value pair", "error", err)
		return err
	}
	m.logger.DebugContext(ctx, "put key-value pair", "key", key)
	return nil
}

// Get retrieves a value by key.
func (m *KVManager) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ValidateKey(key); err != nil {
		return nil, fmt.Errorf("invalid key: %w", err)
	}

	entry, err := m.kv.Get(ctx, key)
	if err != nil {
		m.logger.ErrorContext(ctx, "failed to get key-value pair", "error", err)
		return nil, err
	}
	m.logger.DebugContext(ctx, "got key-value pair", "key", key)
	return entry.Value(), nil
}

// Delete removes a key-value pair.
func (m *KVManager) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}

	if err := m.kv.Delete(ctx, key); err != nil {
		m.logger.ErrorContext(ctx, "failed to delete key-value pair", "error", err)
		return err
	}
	m.logger.DebugContext(ctx, "deleted key-value pair", "key", key)
	return nil
}
