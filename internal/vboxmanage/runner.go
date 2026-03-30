// Package vboxmanage provides a Go wrapper around the VBoxManage CLI tool.
// It offers a mockable Driver interface for executing VBoxManage commands and
// typed functions for common VirtualBox operations.
package vboxmanage

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Driver defines an interface for executing VBoxManage commands.
// It can be mocked in tests to avoid requiring an actual VirtualBox installation.
type Driver interface {
	// Execute runs a VBoxManage command with the given arguments and returns
	// the captured stdout, stderr, and any error.
	Execute(args ...string) (stdout string, stderr string, err error)

	// ExecuteContext runs a VBoxManage command with context support for
	// cancellation and timeouts.
	ExecuteContext(ctx context.Context, args ...string) (stdout string, stderr string, err error)
}

// ExitError wraps an error from VBoxManage execution, providing the exit code
// and captured output for diagnostic purposes.
type ExitError struct {
	Command  string
	ExitCode int
	Stdout   string
	Stderr   string
}

// Error returns a human-readable description of the VBoxManage execution failure.
func (e *ExitError) Error() string {
	return fmt.Sprintf("VBoxManage %s failed with exit code %d: %s", e.Command, e.ExitCode, strings.TrimSpace(e.Stderr))
}

// VBoxManageDriver is the real implementation of Driver that locates and
// executes the VBoxManage binary on the host system.
type VBoxManageDriver struct {
	// VBoxManagePath is the path to the VBoxManage binary.
	// If empty, it will be auto-detected from PATH and common install locations.
	VBoxManagePath string
}

// detectVBoxManage locates the VBoxManage binary by searching the system PATH
// and common installation directories on Linux, macOS, and Windows.
func detectVBoxManage() (string, error) {
	// Try PATH first
	path, err := exec.LookPath("VBoxManage")
	if err == nil {
		return path, nil
	}

	// Common install locations by OS
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		candidates = []string{
			`C:\Program Files\Oracle\VirtualBox\VBoxManage.exe`,
			`C:\Program Files (x86)\Oracle\VirtualBox\VBoxManage.exe`,
		}
	case "darwin":
		candidates = []string{
			"/usr/local/bin/VBoxManage",
			"/opt/homebrew/bin/VBoxManage",
			"/Applications/VirtualBox.app/Contents/MacOS/VBoxManage",
		}
	default: // linux and others
		candidates = []string{
			"/usr/bin/VBoxManage",
			"/usr/local/bin/VBoxManage",
		}
	}

	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c, nil
		}
	}

	return "", fmt.Errorf("VBoxManage binary not found in PATH or common install locations")
}

// getPath returns the configured VBoxManage path or auto-detects it.
func (d *VBoxManageDriver) getPath() (string, error) {
	if d.VBoxManagePath != "" {
		return d.VBoxManagePath, nil
	}
	path, err := detectVBoxManage()
	if err != nil {
		return "", err
	}
	d.VBoxManagePath = path
	return path, nil
}

// Execute runs a VBoxManage command with the given arguments.
func (d *VBoxManageDriver) Execute(args ...string) (string, string, error) {
	return d.ExecuteContext(context.Background(), args...)
}

// ExecuteContext runs a VBoxManage command with context support for cancellation.
func (d *VBoxManageDriver) ExecuteContext(ctx context.Context, args ...string) (string, string, error) {
	vboxManagePath, err := d.getPath()
	if err != nil {
		return "", "", err
	}

	cmd := exec.CommandContext(ctx, vboxManagePath, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if err != nil {
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}

		cmdStr := strings.Join(args, " ")
		return stdout, stderr, &ExitError{
			Command:  cmdStr,
			ExitCode: exitCode,
			Stdout:   stdout,
			Stderr:   stderr,
		}
	}

	return stdout, stderr, nil
}
