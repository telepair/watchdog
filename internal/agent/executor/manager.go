package executor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/telepair/watchdog/pkg/logger"
)

// Manager manages command execution
type Manager struct {
	agentID   string
	listener  MessageListener
	publisher ResultPublisher
	handlers  []CommandHandler
	logger    *slog.Logger

	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	mu      sync.RWMutex
}

// NewManager creates a new executor manager
func NewManager(agentID string, listener MessageListener, publisher ResultPublisher) *Manager {
	m := &Manager{
		agentID:   agentID,
		listener:  listener,
		publisher: publisher,
		handlers:  make([]CommandHandler, 0),
		logger:    logger.ComponentLogger("agent.executor"),
	}

	// Register default handlers
	m.RegisterHandler(NewShellCommandHandler())
	m.RegisterHandler(NewScriptCommandHandler())

	return m
}

// Start starts the executor
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("executor already running")
	}

	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.running = true

	m.logger.Info("starting command executor", "agent_id", m.agentID)

	// Start listening for commands
	if err := m.listener.Subscribe(m.ctx, m.agentID, m.handleCommand); err != nil {
		m.running = false
		return fmt.Errorf("failed to subscribe to mailbox: %w", err)
	}

	return nil
}

// Stop stops the executor
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.logger.Info("stopping command executor")

	// Unsubscribe from mailbox
	if err := m.listener.Unsubscribe(m.agentID); err != nil {
		m.logger.Error("failed to unsubscribe from mailbox", "error", err)
	}

	m.cancel()
	m.running = false

	return nil
}

// IsRunning returns whether the executor is running
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// RegisterHandler registers a new command handler
func (m *Manager) RegisterHandler(handler CommandHandler) {
	m.handlers = append(m.handlers, handler)
}

// handleCommand handles an incoming command
func (m *Manager) handleCommand(command *Command) {
	m.logger.Debug("received command",
		"id", command.ID,
		"type", command.Type,
		"command", command.Command,
	)

	// Find appropriate handler
	var handler CommandHandler
	for _, h := range m.handlers {
		if h.CanHandle(command) {
			handler = h
			break
		}
	}

	if handler == nil {
		m.logger.Error("no handler found for command type", "type", command.Type, "id", command.ID)
		m.publishErrorResult(command, fmt.Errorf("unsupported command type: %s", command.Type))
		return
	}

	// Execute command
	result, err := handler.Execute(m.ctx, command)
	if err != nil {
		m.logger.Error("failed to execute command", "id", command.ID, "error", err)
		m.publishErrorResult(command, err)
		return
	}

	// Publish result
	if err := m.publisher.PublishResult(m.ctx, m.agentID, result); err != nil {
		m.logger.Error("failed to publish result", "id", command.ID, "error", err)
	} else {
		m.logger.Debug("published command result", "id", command.ID, "success", result.Success)
	}
}

// publishErrorResult publishes an error result for a failed command
func (m *Manager) publishErrorResult(command *Command, err error) {
	result := &Result{
		ID:      command.ID,
		Command: command,
		Success: false,
		Error:   err.Error(),
	}

	if publishErr := m.publisher.PublishResult(m.ctx, m.agentID, result); publishErr != nil {
		m.logger.Error("failed to publish error result", "id", command.ID, "error", publishErr)
	}
}
