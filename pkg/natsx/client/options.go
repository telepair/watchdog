package client

import (
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

// OptionsBuilder builds NATS connection options.
type OptionsBuilder struct {
	client *Client
}

// NewOptionsBuilder creates a new options builder.
func NewOptionsBuilder(client *Client) *OptionsBuilder {
	return &OptionsBuilder{client: client}
}

// BuildNATSOptions constructs NATS connection options.
func (ob *OptionsBuilder) BuildNATSOptions(config *Config) ([]nats.Option, error) {
	opts := []nats.Option{
		nats.Timeout(config.ConnectTimeout),
		nats.MaxReconnects(config.MaxReconnects),
		nats.ReconnectWait(config.ReconnectWait),
		nats.Name(config.Name),
	}

	opts = append(opts, ob.buildEventHandlers()...)

	authOpts, err := ob.buildAuthOptions(config)
	if err != nil {
		return nil, err
	}
	opts = append(opts, authOpts...)

	opts = append(opts, ob.buildTLSOptions(config)...)

	return opts, nil
}

// buildEventHandlers creates NATS event handlers.
func (ob *OptionsBuilder) buildEventHandlers() []nats.Option {
	return []nats.Option{
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				ob.client.logger.Warn("NATS disconnected", "error", err)
			}
			if ob.client.metrics != nil {
				ob.client.metrics.RecordDisconnection()
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			ob.client.logger.Info("NATS reconnected", "url", nc.ConnectedUrl())
			if ob.client.metrics != nil {
				ob.client.metrics.RecordReconnection()
			}
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			ob.client.logger.Warn("NATS connection closed")
			if ob.client.metrics != nil {
				ob.client.metrics.RecordConnectionClosed()
			}
		}),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			if err != nil {
				ob.client.logger.Error("NATS async error", "error", err)
				if ob.client.metrics != nil {
					ob.client.metrics.RecordError()
				}
			}
		}),
	}
}

// buildAuthOptions creates authentication options.
func (ob *OptionsBuilder) buildAuthOptions(config *Config) ([]nats.Option, error) {
	var opts []nats.Option

	if config.Token != "" {
		opts = append(opts, nats.Token(config.Token))
		return opts, nil
	}

	if config.JWT != "" {
		return ob.buildJWTAuthOptions(config)
	}

	if config.NKey != "" {
		return ob.buildNKeyAuthOptions(config)
	}

	return opts, nil
}

// buildJWTAuthOptions creates JWT authentication options.
func (ob *OptionsBuilder) buildJWTAuthOptions(config *Config) ([]nats.Option, error) {
	if config.NKey == "" {
		return nil, errors.New("JWT authentication requires NKey for signing")
	}

	kp, err := nkeys.FromSeed([]byte(config.NKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create keypair from nkey: %w", err)
	}

	return []nats.Option{
		nats.UserJWT(
			func() (string, error) { return config.JWT, nil },
			func(nonce []byte) ([]byte, error) { return kp.Sign(nonce) },
		),
	}, nil
}

// buildNKeyAuthOptions creates NKey authentication options.
func (ob *OptionsBuilder) buildNKeyAuthOptions(config *Config) ([]nats.Option, error) {
	nkeyOpt, err := nats.NkeyOptionFromSeed(config.NKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create nkey option: %w", err)
	}
	return []nats.Option{nkeyOpt}, nil
}

// buildTLSOptions creates TLS options.
func (ob *OptionsBuilder) buildTLSOptions(config *Config) []nats.Option {
	if !config.EnableTLS {
		return nil
	}

	if config.TLSSkipVerify {
		ob.client.logger.Warn("TLS certificate verification is DISABLED - this is INSECURE for production use",
			"security_risk", "man_in_the_middle_attacks",
			"recommendation", "enable certificate verification for production")
		tlsConfig := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // intentional skip verify when configured
		return []nats.Option{nats.Secure(tlsConfig)}
	}

	return []nats.Option{nats.Secure()}
}
