package executor

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Executor handles command execution with permission checking
type Executor struct {
	permissions *PermissionManager
	timeout     time.Duration
}

// NewExecutor creates a new command executor with default settings
func NewExecutor() *Executor {
	return &Executor{
		permissions: NewPermissionManager(),
		timeout:     30 * time.Second, // Default 30 second timeout
	}
}

// ExecutionResult contains the result of a command execution
type ExecutionResult struct {
	Command  string
	Output   string
	Error    error
	ExitCode int
	Duration time.Duration
}

// Execute runs a shell command and returns the result
func (e *Executor) Execute(ctx context.Context, command string) (*ExecutionResult, error) {
	start := time.Now()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Execute command using shell
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()

	result := &ExecutionResult{
		Command:  command,
		Output:   string(output),
		Error:    err,
		Duration: time.Since(start),
	}

	// Extract exit code if available
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else if err == nil {
		result.ExitCode = 0
	} else {
		result.ExitCode = -1
	}

	return result, nil
}

// GetPermissionManager returns the permission manager
func (e *Executor) GetPermissionManager() *PermissionManager {
	return e.permissions
}

// SetTimeout sets the command execution timeout
func (e *Executor) SetTimeout(timeout time.Duration) {
	e.timeout = timeout
}

// FormatResult formats an execution result for display
func (r *ExecutionResult) FormatResult() string {
	if r.Error != nil && r.ExitCode != 0 {
		return fmt.Sprintf("Command failed with exit code %d:\n%s", r.ExitCode, r.Output)
	}
	return r.Output
}

// IsSuccess returns true if the command executed successfully
func (r *ExecutionResult) IsSuccess() bool {
	return r.ExitCode == 0
}
