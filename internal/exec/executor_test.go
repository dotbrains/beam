package exec

import (
	"context"
	"strings"
	"testing"
)

// MockExecutor implements CommandExecutor for testing.
type MockExecutor struct {
	RunFunc          func(ctx context.Context, name string, args ...string) (string, error)
	RunWithStdinFunc func(ctx context.Context, stdin string, name string, args ...string) (string, error)
}

func (m *MockExecutor) Run(ctx context.Context, name string, args ...string) (string, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, name, args...)
	}
	return "", nil
}

func (m *MockExecutor) RunWithStdin(ctx context.Context, stdin string, name string, args ...string) (string, error) {
	if m.RunWithStdinFunc != nil {
		return m.RunWithStdinFunc(ctx, stdin, name, args...)
	}
	return "", nil
}

func TestNewRealExecutor(t *testing.T) {
	e := NewRealExecutor()
	if e == nil {
		t.Fatal("NewRealExecutor returned nil")
	}
}

func TestRealExecutor_Run(t *testing.T) {
	e := NewRealExecutor()
	ctx := context.Background()

	out, err := e.Run(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("output = %q, want to contain 'hello'", out)
	}
}

func TestRealExecutor_Run_Error(t *testing.T) {
	e := NewRealExecutor()
	ctx := context.Background()

	_, err := e.Run(ctx, "nonexistent-command-abc123")
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

func TestRealExecutor_RunWithStdin(t *testing.T) {
	e := NewRealExecutor()
	ctx := context.Background()

	out, err := e.RunWithStdin(ctx, "hello from stdin", "cat")
	if err != nil {
		t.Fatalf("RunWithStdin error: %v", err)
	}
	if !strings.Contains(out, "hello from stdin") {
		t.Errorf("output = %q, want to contain 'hello from stdin'", out)
	}
}

func TestMockExecutor(t *testing.T) {
	mock := &MockExecutor{
		RunFunc: func(ctx context.Context, name string, args ...string) (string, error) {
			return "mocked output", nil
		},
	}

	out, err := mock.Run(context.Background(), "anything")
	if err != nil {
		t.Fatalf("mock Run error: %v", err)
	}
	if out != "mocked output" {
		t.Errorf("output = %q, want 'mocked output'", out)
	}
}

func TestMockExecutor_Defaults(t *testing.T) {
	mock := &MockExecutor{}

	out, err := mock.Run(context.Background(), "anything")
	if err != nil {
		t.Fatalf("mock Run error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}

	out, err = mock.RunWithStdin(context.Background(), "stdin", "anything")
	if err != nil {
		t.Fatalf("mock RunWithStdin error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}
