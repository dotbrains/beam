package exec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// CommandExecutor abstracts command execution for testability.
type CommandExecutor interface {
	// Run executes a command and returns combined stdout output.
	Run(ctx context.Context, name string, args ...string) (string, error)

	// RunWithStdin executes a command with stdin and returns stdout.
	RunWithStdin(ctx context.Context, stdin string, name string, args ...string) (string, error)
}

// RealExecutor shells out to real commands.
type RealExecutor struct{}

func NewRealExecutor() *RealExecutor {
	return &RealExecutor{}
}

func (e *RealExecutor) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func (e *RealExecutor) RunWithStdin(ctx context.Context, stdin string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewBufferString(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
