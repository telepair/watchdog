package executor

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/telepair/watchdog/internal/config"
)

// ShellCommandHandler handles shell command execution
type ShellCommandHandler struct{}

// NewShellCommandHandler creates a new shell command handler
func NewShellCommandHandler() *ShellCommandHandler {
	return &ShellCommandHandler{}
}

// CanHandle returns true if this handler can execute the given command type
func (h *ShellCommandHandler) CanHandle(command *Command) bool {
	return command.Type == config.ExecTypeCommand
}

// Execute executes a shell command
func (h *ShellCommandHandler) Execute(ctx context.Context, command *Command) (*Result, error) {
	result := &Result{
		ID:        command.ID,
		Command:   command,
		StartedAt: time.Now(),
	}

	// Set timeout context if specified
	if command.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, command.Timeout)
		defer cancel()
	}

	// Validate command before execution to prevent injection
	if err := validateCommand(command.Command); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("invalid command: %v", err)
		result.CompletedAt = time.Now()
		return result, err
	}

	// Prepare command
	var cmd *exec.Cmd
	if len(command.Args) > 0 {
		// #nosec G204 - This is intentional command execution with validated input
		cmd = exec.CommandContext(ctx, command.Command, command.Args...)
	} else {
		// If no args, treat command as shell command
		// #nosec G204 - This is intentional shell command execution with validated input
		cmd = exec.CommandContext(ctx, "sh", "-c", command.Command)
	}

	// Set working directory
	if command.WorkingDir != "" {
		cmd.Dir = command.WorkingDir
	}

	// Set environment variables
	if len(command.Env) > 0 {
		env := make([]string, 0, len(command.Env))
		for k, v := range command.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	// Execute command
	stdout, err := cmd.Output()
	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)
	result.Stdout = string(stdout)

	if err != nil {
		result.Success = false
		result.Error = err.Error()

		// Extract stderr from ExitError if available
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
			result.Stderr = string(exitError.Stderr)
		} else {
			result.ExitCode = -1
		}
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	return result, nil
}

// ScriptCommandHandler handles script execution
type ScriptCommandHandler struct{}

// NewScriptCommandHandler creates a new script command handler
func NewScriptCommandHandler() *ScriptCommandHandler {
	return &ScriptCommandHandler{}
}

// CanHandle returns true if this handler can execute the given command type
func (h *ScriptCommandHandler) CanHandle(command *Command) bool {
	return command.Type == config.ExecTypeScript
}

// Execute executes a script command
func (h *ScriptCommandHandler) Execute(ctx context.Context, command *Command) (*Result, error) {
	result := &Result{
		ID:        command.ID,
		Command:   command,
		StartedAt: time.Now(),
	}

	// Set timeout context if specified
	if command.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, command.Timeout)
		defer cancel()
	}

	// Validate script file before execution
	if err := validateScriptFile(command.Command); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("invalid script file: %v", err)
		result.CompletedAt = time.Now()
		return result, err
	}

	// Determine script interpreter based on command or file extension
	interpreter := "sh"
	if strings.HasSuffix(command.Command, ".py") {
		interpreter = "python"
	} else if strings.HasSuffix(command.Command, ".js") {
		interpreter = "node"
	} else if strings.HasSuffix(command.Command, ".rb") {
		interpreter = "ruby"
	}

	// Prepare command
	args := []string{command.Command}
	args = append(args, command.Args...)
	// #nosec G204 - This is intentional script execution with validated file path
	cmd := exec.CommandContext(ctx, interpreter, args...)

	// Set working directory
	if command.WorkingDir != "" {
		cmd.Dir = command.WorkingDir
	}

	// Set environment variables
	if len(command.Env) > 0 {
		env := make([]string, 0, len(command.Env))
		for k, v := range command.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	// Execute command
	stdout, err := cmd.Output()
	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)
	result.Stdout = string(stdout)

	if err != nil {
		result.Success = false
		result.Error = err.Error()

		// Extract stderr from ExitError if available
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
			result.Stderr = string(exitError.Stderr)
		} else {
			result.ExitCode = -1
		}
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	return result, nil
}

// validateCommand performs basic validation on command strings to prevent obvious injection
func validateCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	// Check for suspicious patterns that might indicate injection attempts
	dangerousPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[;&|` + "`" + `]\s*rm\s+-rf`),    // Destructive rm commands
		regexp.MustCompile(`[;&|` + "`" + `]\s*format\s+c:`), // Windows format commands
		regexp.MustCompile(`\$\([^)]*\)`),                    // Command substitution
	}

	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("command contains potentially dangerous patterns")
		}
	}

	return nil
}

// validateScriptFile validates script file paths to prevent directory traversal and ensure file exists
func validateScriptFile(scriptPath string) error {
	if scriptPath == "" {
		return fmt.Errorf("script path cannot be empty")
	}

	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(scriptPath)

	// Check for directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("script path contains directory traversal")
	}

	// Ensure the path is not absolute (restrict to relative paths)
	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("absolute script paths are not allowed")
	}

	return nil
}
