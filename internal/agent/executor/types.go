package executor

import (
	"context"
	"time"
)

// Command represents a command to be executed
type Command struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`        // cmd, script, task, job
	Command    string            `json:"command"`     // command to execute
	Args       []string          `json:"args"`        // command arguments
	WorkingDir string            `json:"working_dir"` // working directory
	Env        map[string]string `json:"env"`         // environment variables
	Timeout    time.Duration     `json:"timeout"`     // execution timeout
	CreatedAt  time.Time         `json:"created_at"`
}

// Result represents the result of command execution
type Result struct {
	ID          string        `json:"id"`
	Command     *Command      `json:"command"`
	Success     bool          `json:"success"`
	ExitCode    int           `json:"exit_code"`
	Stdout      string        `json:"stdout"`
	Stderr      string        `json:"stderr"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
}

// CommandHandler defines interface for handling different command types
type CommandHandler interface {
	CanHandle(command *Command) bool
	Execute(ctx context.Context, command *Command) (*Result, error)
}

// ResultPublisher defines interface for publishing execution results
type ResultPublisher interface {
	PublishResult(ctx context.Context, agentID string, result *Result) error
}

// MessageListener defines interface for listening to incoming commands
type MessageListener interface {
	Subscribe(ctx context.Context, agentID string, handler func(command *Command)) error
	Unsubscribe(agentID string) error
}

// Executor represents the command executor component
type Executor interface {
	Start() error
	Stop() error
	IsRunning() bool
	RegisterHandler(handler CommandHandler)
}
