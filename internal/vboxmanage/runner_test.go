package vboxmanage

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// MockDriver records calls and returns preset responses for testing.
type MockDriver struct {
	// Calls records every invocation as a slice of argument slices.
	Calls [][]string

	// Stdout is returned as the stdout value for each call.
	Stdout string

	// Stderr is returned as the stderr value for each call.
	Stderr string

	// Err is returned as the error value for each call.
	Err error

	// StdoutFunc, if set, overrides Stdout. It receives the arguments and returns stdout.
	StdoutFunc func(args []string) string

	// ErrFunc, if set, overrides Err. It receives the arguments and returns an error.
	ErrFunc func(args []string) error
}

// Execute records the call and returns the preset response.
func (m *MockDriver) Execute(args ...string) (string, string, error) {
	return m.ExecuteContext(context.Background(), args...)
}

// ExecuteContext records the call and returns the preset response.
func (m *MockDriver) ExecuteContext(_ context.Context, args ...string) (string, string, error) {
	m.Calls = append(m.Calls, args)

	stdout := m.Stdout
	if m.StdoutFunc != nil {
		stdout = m.StdoutFunc(args)
	}

	err := m.Err
	if m.ErrFunc != nil {
		err = m.ErrFunc(args)
	}

	return stdout, m.Stderr, err
}

func TestMockDriverRecordsCalls(t *testing.T) {
	mock := &MockDriver{Stdout: "ok"}

	stdout, stderr, err := mock.Execute("showvminfo", "test-vm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "ok" {
		t.Errorf("expected stdout 'ok', got %q", stdout)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got %q", stderr)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.Calls))
	}
	if strings.Join(mock.Calls[0], " ") != "showvminfo test-vm" {
		t.Errorf("unexpected call args: %v", mock.Calls[0])
	}
}

func TestMockDriverReturnsError(t *testing.T) {
	mock := &MockDriver{
		Stderr: "error details",
		Err:    fmt.Errorf("command failed"),
	}

	_, stderr, err := mock.Execute("badcommand")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if stderr != "error details" {
		t.Errorf("expected stderr 'error details', got %q", stderr)
	}
}

func TestMockDriverWithStdoutFunc(t *testing.T) {
	mock := &MockDriver{
		StdoutFunc: func(args []string) string {
			if len(args) > 0 && args[0] == "list" {
				return "vm1\nvm2\n"
			}
			return ""
		},
	}

	stdout, _, err := mock.Execute("list", "vms")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "vm1\nvm2\n" {
		t.Errorf("unexpected stdout: %q", stdout)
	}
}

func TestMockDriverMultipleCalls(t *testing.T) {
	mock := &MockDriver{Stdout: "result"}

	mock.Execute("cmd1")
	mock.Execute("cmd2", "arg")
	mock.Execute("cmd3", "a", "b")

	if len(mock.Calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(mock.Calls))
	}
}

func TestExitErrorFormat(t *testing.T) {
	e := &ExitError{
		Command:  "showvminfo test",
		ExitCode: 1,
		Stdout:   "",
		Stderr:   "VM not found\n",
	}

	msg := e.Error()
	if !strings.Contains(msg, "exit code 1") {
		t.Errorf("error message should contain exit code: %s", msg)
	}
	if !strings.Contains(msg, "VM not found") {
		t.Errorf("error message should contain stderr: %s", msg)
	}
}

func TestMockDriverExecuteContext(t *testing.T) {
	mock := &MockDriver{Stdout: "context-result"}
	ctx := context.Background()

	stdout, _, err := mock.ExecuteContext(ctx, "test", "arg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "context-result" {
		t.Errorf("expected 'context-result', got %q", stdout)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.Calls))
	}
}
