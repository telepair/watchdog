package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	// DefaultConnectTimeout is the default timeout for connecting to NATS.
	DefaultConnectTimeout = 2 * time.Second
	// DefaultReconnectWait is the default wait time between reconnection attempts.
	DefaultReconnectWait = 2 * time.Second
	// DefaultPort is the default NATS server port.
	DefaultPort = 4222
	// UnlimitedReconnects indicates unlimited reconnection attempts.
	UnlimitedReconnects = -1
	// DrainTimeout is the timeout for draining connections during close.
	DrainTimeout = 5 * time.Second
)

// Config holds NATS client configuration.
type Config struct {
	Name           string        `yaml:"name"            json:"name"`
	URLs           []string      `yaml:"urls"            json:"urls"`
	Token          string        `yaml:"token"           json:"-"`
	NKey           string        `yaml:"nkey"            json:"-"`
	JWT            string        `yaml:"jwt"             json:"-"`
	ConnectTimeout time.Duration `yaml:"connect_timeout" json:"connect_timeout"`
	MaxReconnects  int           `yaml:"max_reconnects"  json:"max_reconnects"`
	ReconnectWait  time.Duration `yaml:"reconnect_wait"  json:"reconnect_wait"`
	EnableTLS      bool          `yaml:"enable_tls"      json:"enable_tls"`
	// TLSSkipVerify skips TLS certificate verification.
	// WARNING: This is insecure and should NEVER be used in production.
	// Only enable this for testing or development environments.
	// When enabled, the client is vulnerable to man-in-the-middle attacks.
	TLSSkipVerify bool `yaml:"tls_skip_verify" json:"tls_skip_verify"`
}

// Validate validates the client configuration.
func (c *Config) Validate() error {
	if c.Name == "" {
		c.Name = "natsx.client"
	}

	if len(c.URLs) == 0 {
		c.URLs = []string{"nats://" + net.JoinHostPort("localhost", strconv.Itoa(DefaultPort))}
	}

	// Validate all URLs
	for i, u := range c.URLs {
		if err := ValidateNATSURL(u); err != nil {
			return fmt.Errorf("invalid URL at index %d: %w", i, err)
		}
	}

	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = DefaultConnectTimeout
	}

	if c.ReconnectWait <= 0 {
		c.ReconnectWait = DefaultReconnectWait
	}

	return nil
}

// DefaultConfig returns a default NATS configuration.
func DefaultConfig() *Config {
	return &Config{
		Name:           "natsx.client",
		URLs:           []string{"nats://" + net.JoinHostPort("localhost", strconv.Itoa(DefaultPort))},
		ConnectTimeout: DefaultConnectTimeout,
		MaxReconnects:  UnlimitedReconnects,
		ReconnectWait:  DefaultReconnectWait,
		EnableTLS:      false,
		TLSSkipVerify:  false,
	}
}

// Client wraps a NATS connection with JetStream support.
type Client struct {
	name          string
	conn          *nats.Conn
	js            jetstream.JetStream
	config        *Config
	logger        *slog.Logger
	mu            sync.RWMutex
	closed        bool
	subscriptions []*nats.Subscription
	subMu         sync.RWMutex
	metrics       *Metrics
}

// NewClient creates a new NATS client.
func NewClient(config *Config) (*Client, error) {
	return NewClientWithMetrics(config, nil)
}

// NewClientWithMetrics creates a new NATS client with optional metrics collector.
func NewClientWithMetrics(config *Config, metricsCollector MetricsCollector) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	client := &Client{
		name:    config.Name,
		config:  config,
		logger:  slog.Default().With("component", "natsx.client"),
		metrics: createMetrics(metricsCollector, config.Name),
	}

	optionsBuilder := NewOptionsBuilder(client)
	opts, err := optionsBuilder.BuildNATSOptions(config)
	if err != nil {
		return nil, err
	}

	if connectErr := client.connectToNATS(config, opts); connectErr != nil {
		return nil, connectErr
	}

	return client, nil
}

// createMetrics initializes metrics collector or noop collector.
func createMetrics(metricsCollector MetricsCollector, clientName string) *Metrics {
	if metricsCollector != nil {
		return NewMetrics(metricsCollector, clientName)
	}
	return NewNoopMetrics(clientName)
}

// connectToNATS establishes connection to NATS and creates JetStream context.
func (c *Client) connectToNATS(config *Config, opts []nats.Option) error {
	seedURL := config.URLs[0]
	if len(config.URLs) > 1 {
		seedURL = strings.Join(config.URLs, ",")
	}

	nc, err := nats.Connect(seedURL, opts...)
	if err != nil {
		c.logger.Error("failed to connect to NATS", "error", err)
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		c.logger.Error("failed to create JetStream context", "error", err)
		return fmt.Errorf("failed to create JetStream context: %w", err)
	}

	c.conn = nc
	c.js = js

	if c.metrics != nil {
		c.metrics.RecordConnection()
	}

	return nil
}

// Close closes the NATS connection and cleans up resources.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	// Close all subscriptions first
	c.closeSubscriptions()

	// Close the connection with timeout
	if c.conn != nil {
		// Use a timeout channel for drain to avoid indefinite blocking
		done := make(chan error, 1)
		go func() {
			done <- c.conn.Drain()
		}()

		select {
		case err := <-done:
			if err != nil {
				c.logger.Warn("failed to drain connection", "error", err)
			}
		case <-time.After(DrainTimeout):
			c.logger.Warn("connection drain timeout, forcing close")
		}

		// Always close the connection regardless of drain result
		c.conn.Close()
	}

	// Update connection metrics
	if c.metrics != nil {
		c.metrics.RecordConnectionClosed()
	}

	c.closed = true
	c.logger.Debug("client closed")
	return nil
}

// Conn returns the underlying NATS connection.
func (c *Client) Conn() *nats.Conn {
	return c.conn
}

// JetStream returns the JetStream context.
func (c *Client) JetStream() jetstream.JetStream {
	return c.js
}

// IsConnected returns true if the client is connected to NATS.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || c.conn == nil {
		return false
	}
	return c.conn.IsConnected()
}

// Publish publishes data to the specified subject.
func (c *Client) Publish(subject string, data []byte) error {
	return c.PublishContext(context.Background(), subject, data)
}

// PublishContext publishes data with context, integrating metrics and tracing.
func (c *Client) PublishContext(ctx context.Context, subject string, data []byte) error {
	if err := ValidateSubject(subject); err != nil {
		return c.ErrorWithMetrics(WrapValidationError("subject", err))
	}

	if err := c.CheckClientState(); err != nil {
		return err
	}

	start := time.Now()
	c.logger.DebugContext(ctx, "publishing data to subject", "subject", subject)

	err := c.conn.Publish(subject, data)

	// Honor context completion semantics (best effort)
	if err == nil {
		err = c.handleContextTimeout(ctx)
	}

	if c.metrics != nil {
		if err != nil {
			c.metrics.RecordError()
		} else {
			c.metrics.RecordPublish(len(data), time.Since(start))
		}
	}

	return err
}

// Message represents a message to be published
type Message struct {
	Subject string
	Data    []byte
}

// PublishBatch publishes multiple messages efficiently in a batch
func (c *Client) PublishBatch(ctx context.Context, messages []Message) error {
	return c.PublishBatchWithFlush(ctx, messages, true)
}

// PublishBatchWithFlush publishes multiple messages with optional flush control
func (c *Client) PublishBatchWithFlush(ctx context.Context, messages []Message, flush bool) error {
	if len(messages) == 0 {
		return nil
	}

	if err := c.CheckClientState(); err != nil {
		return err
	}

	start := time.Now()
	totalBytes := 0
	errorCount := 0

	// Validate all subjects first
	for i, msg := range messages {
		if err := ValidateSubject(msg.Subject); err != nil {
			return c.ErrorWithMetrics(WrapValidationError(fmt.Sprintf("message[%d].subject", i), err))
		}
		totalBytes += len(msg.Data)
	}

	c.logger.DebugContext(ctx, "publishing batch messages",
		"count", len(messages),
		"total_bytes", totalBytes)

	// Publish all messages without flushing each one
	for _, msg := range messages {
		if err := c.conn.Publish(msg.Subject, msg.Data); err != nil {
			errorCount++
			c.logger.WarnContext(ctx, "failed to publish message in batch",
				"subject", msg.Subject,
				"error", err)
		}
	}

	// Single flush for all messages if requested
	var flushErr error
	if flush && errorCount < len(messages) {
		flushErr = c.handleContextTimeout(ctx)
	}

	// Record metrics
	if c.metrics != nil {
		if errorCount > 0 {
			c.metrics.RecordError()
		}
		if errorCount < len(messages) {
			// Record successful publishes
			c.metrics.RecordPublish(totalBytes, time.Since(start))
		}
	}

	if errorCount == len(messages) {
		return fmt.Errorf("all %d messages failed to publish", len(messages))
	} else if errorCount > 0 {
		return fmt.Errorf("%d out of %d messages failed to publish", errorCount, len(messages))
	}

	return flushErr
}

// handleContextTimeout handles context timeout and flushing logic.
func (c *Client) handleContextTimeout(ctx context.Context) error {
	// Check if context is already canceled/expired
	if err := ctx.Err(); err != nil {
		return err
	}

	// Handle deadline-based timeout
	deadline, ok := ctx.Deadline()
	if !ok {
		// No deadline, just flush with default timeout
		return c.conn.Flush()
	}

	timeRemaining := time.Until(deadline)
	if timeRemaining <= 0 {
		return context.DeadlineExceeded
	}

	// Flush with remaining time
	if flushErr := c.conn.FlushTimeout(timeRemaining); flushErr != nil {
		return fmt.Errorf("flush timeout: %w", flushErr)
	}
	return nil
}

// Subscribe subscribes to messages on the specified subject.
func (c *Client) Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return c.SubscribeContext(context.Background(), subject, handler)
}

// SubscribeContext subscribes with context, integrating metrics and tracing.
func (c *Client) SubscribeContext(
	ctx context.Context,
	subject string,
	handler nats.MsgHandler,
) (*nats.Subscription, error) {
	if err := ValidateSubject(subject); err != nil {
		return nil, c.ErrorWithMetrics(WrapValidationError("subject", err))
	}

	if err := c.CheckClientState(); err != nil {
		return nil, err
	}

	// Build handler with metrics
	handlerWithMetrics := func(msg *nats.Msg) {
		// Metrics for receive
		if c.metrics != nil {
			c.metrics.RecordReceive(len(msg.Data))
		}

		handler(msg)
	}

	c.logger.DebugContext(context.Background(), "subscribing to subject", "subject", subject)
	sub, err := c.conn.Subscribe(subject, handlerWithMetrics)

	if err != nil {
		return nil, c.ErrorWithMetrics(err)
	}

	// Add to managed subscriptions
	c.subMu.Lock()
	c.subscriptions = append(c.subscriptions, sub)
	subCount := len(c.subscriptions)
	c.subMu.Unlock()

	// Update subscription count
	if c.metrics != nil {
		c.metrics.SetSubscriptionCount(subCount)
	}

	// Auto-unsubscribe when context is done
	if ctx != nil {
		go func() {
			<-ctx.Done()
			_ = c.Unsubscribe(sub)
		}()
	}

	return sub, nil
}

// Unsubscribe removes a subscription and cleans it up.
func (c *Client) Unsubscribe(sub *nats.Subscription) error {
	if sub == nil {
		return nil
	}

	// Remove from managed subscriptions
	c.subMu.Lock()
	for i, s := range c.subscriptions {
		if s == sub {
			c.subscriptions = append(c.subscriptions[:i], c.subscriptions[i+1:]...)
			break
		}
	}
	subCount := len(c.subscriptions)
	c.subMu.Unlock()

	// Update subscription count metrics
	if c.metrics != nil {
		c.metrics.SetSubscriptionCount(subCount)
	}

	err := sub.Unsubscribe()
	if err != nil {
		return c.ErrorWithMetrics(err)
	}
	return nil
}

// closeSubscriptions closes all managed subscriptions.
func (c *Client) closeSubscriptions() {
	c.subMu.Lock()
	defer c.subMu.Unlock()

	// Add panic recovery for concurrent subscription cleanup
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("panic during subscription cleanup", "panic", r)
		}
	}()

	for _, sub := range c.subscriptions {
		if sub != nil {
			// Protect individual unsubscribe operations
			func(subscription *nats.Subscription) {
				defer func() {
					if r := recover(); r != nil {
						c.logger.Warn("panic during individual unsubscribe", "panic", r)
					}
				}()
				if err := subscription.Unsubscribe(); err != nil {
					c.logger.Warn("failed to unsubscribe", "error", err)
				}
			}(sub)
		}
	}
	c.subscriptions = nil
	if c.metrics != nil {
		c.metrics.SetSubscriptionCount(0)
	}
}

// GetSubscriptionCount returns the number of active subscriptions.
func (c *Client) GetSubscriptionCount() int {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	return len(c.subscriptions)
}

// HealthCheck performs a basic health check on the NATS connection.
func (c *Client) HealthCheck() error {
	if !c.IsConnected() {
		return errors.New("natsx: not connected")
	}
	return nil
}

// ConnectionStatus represents NATS connection status.
type ConnectionStatus struct {
	Connected     bool      `json:"connected"`
	LastConnected time.Time `json:"last_connected"`
	LastError     string    `json:"last_error,omitempty"`
	Reconnects    int       `json:"reconnects"`
	ServerURL     string    `json:"server_url,omitempty"`
}

// GetMetrics returns the client metrics (for testing or monitoring).
func (c *Client) GetMetrics() *Metrics {
	return c.metrics
}

// IsMetricsEnabled returns whether metrics collection is enabled.
func (c *Client) IsMetricsEnabled() bool {
	return c.metrics != nil
}

// CreateKVManager creates a new KV manager with the given bucket configuration.
func (c *Client) CreateKVManager(config BucketConfig) (*KVManager, error) {
	if err := c.CheckClientState(); err != nil {
		return nil, err
	}
	return NewKVManager(c.js, config)
}

// PutKV stores a key-value pair in the specified bucket.
func (c *Client) PutKV(ctx context.Context, bucketName, key string, value []byte) error {
	if err := c.CheckClientState(); err != nil {
		return err
	}

	kv, err := c.js.KeyValue(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to get KV bucket %s: %w", bucketName, err)
	}

	_, err = kv.Put(ctx, key, value)
	if err != nil {
		return fmt.Errorf("failed to put key-value pair: %w", err)
	}

	return nil
}

// GetKV retrieves a value by key from the specified bucket.
func (c *Client) GetKV(ctx context.Context, bucketName, key string) ([]byte, error) {
	if err := c.CheckClientState(); err != nil {
		return nil, err
	}

	kv, err := c.js.KeyValue(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get KV bucket %s: %w", bucketName, err)
	}

	entry, err := kv.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get key-value pair: %w", err)
	}

	return entry.Value(), nil
}
