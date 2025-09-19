package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"

	"github.com/telepair/watchdog/internal/config"
	"github.com/telepair/watchdog/pkg/natsx/client"
)

// NATSListener listens for incoming commands via NATS
type NATSListener struct {
	client       *client.Client
	subscription *nats.Subscription
	streamCfg    *config.AgentStreamConfig
}

// NewNATSListener creates a new NATS message listener
func NewNATSListener(client *client.Client, sCfg *config.AgentStreamConfig) *NATSListener {
	return &NATSListener{
		client:    client,
		streamCfg: sCfg,
	}
}

// Subscribe subscribes to the agent's mailbox for incoming commands
func (l *NATSListener) Subscribe(ctx context.Context, agentID string, handler func(command *Command)) error {
	subject := l.streamCfg.MailboxSubject(agentID)

	sub, err := l.client.Subscribe(subject, func(msg *nats.Msg) {
		var command Command
		if err := json.Unmarshal(msg.Data, &command); err != nil {
			// Log error but don't fail the whole system
			// TODO: Publish to error subject
			return
		}

		// Call the handler
		handler(&command)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to mailbox: %w", err)
	}

	l.subscription = sub
	return nil
}

// Unsubscribe unsubscribes from the agent's mailbox
func (l *NATSListener) Unsubscribe(agentID string) error {
	if l.subscription != nil {
		if err := l.subscription.Unsubscribe(); err != nil {
			return fmt.Errorf("failed to unsubscribe from mailbox: %w", err)
		}
		l.subscription = nil
	}
	return nil
}

// NATSResultPublisher publishes command execution results via NATS
type NATSResultPublisher struct {
	client    *client.Client
	streamCfg *config.AgentStreamConfig
}

// NewNATSResultPublisher creates a new NATS result publisher
func NewNATSResultPublisher(client *client.Client, scfg *config.AgentStreamConfig) *NATSResultPublisher {
	return &NATSResultPublisher{
		client:    client,
		streamCfg: scfg,
	}
}

// PublishResult publishes a command execution result
func (p *NATSResultPublisher) PublishResult(ctx context.Context, agentID string, result *Result) error {
	subject := p.streamCfg.ExecResultSubject(agentID, result.Command.Type, result.ID)

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := p.client.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish result: %w", err)
	}

	return nil
}
