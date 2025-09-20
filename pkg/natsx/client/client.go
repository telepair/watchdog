package client

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
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
	name   string
	config *Config
	conn   *nats.Conn
	js     jetstream.JetStream
	closed atomic.Bool
	logger *slog.Logger
}

// NewClient creates a new NATS client.
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	client := &Client{
		name:   config.Name,
		config: config,
		logger: slog.Default().With("component", "natsx.client"),
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

	return nil
}

// Close closes the NATS connection and cleans up resources.
func (c *Client) Close() error {
	if c.closed.Load() {
		return nil
	}

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

	c.closed.Store(true)
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
	if c.closed.Load() || c.conn == nil {
		return false
	}
	return c.conn.IsConnected()
}

// HealthCheck performs a basic health check on the NATS connection.
func (c *Client) HealthCheck() error {
	if !c.IsConnected() {
		return errors.New("natsx: not connected")
	}
	return nil
}
