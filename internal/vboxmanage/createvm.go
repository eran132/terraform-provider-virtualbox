package vboxmanage

import (
	"fmt"
	"regexp"
	"strings"
)

// uuidRegexp matches a standard UUID format in VBoxManage output.
var uuidRegexp = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// CreateVM creates a new virtual machine using VBoxManage createvm.
// It returns the UUID assigned to the new VM. If register is true, the VM
// is automatically registered with VirtualBox.
func CreateVM(driver Driver, name string, osType string, basefolder string, register bool) (string, error) {
	args := []string{"createvm", "--name", name}

	if osType != "" {
		args = append(args, "--ostype", osType)
	}
	if basefolder != "" {
		args = append(args, "--basefolder", basefolder)
	}
	if register {
		args = append(args, "--register")
	}

	stdout, _, err := driver.Execute(args...)
	if err != nil {
		return "", fmt.Errorf("createvm %s: %w", name, err)
	}

	uuid := parseUUID(stdout)
	if uuid == "" {
		return "", fmt.Errorf("createvm %s: could not parse UUID from output: %s", name, strings.TrimSpace(stdout))
	}

	return uuid, nil
}

// UnregisterVM unregisters a VM and optionally deletes its files.
func UnregisterVM(driver Driver, nameOrUUID string, deleteFiles bool) error {
	args := []string{"unregistervm", nameOrUUID}
	if deleteFiles {
		args = append(args, "--delete")
	}

	_, _, err := driver.Execute(args...)
	if err != nil {
		return fmt.Errorf("unregistervm %s: %w", nameOrUUID, err)
	}
	return nil
}

// parseUUID extracts the first UUID found in the given text.
func parseUUID(text string) string {
	match := uuidRegexp.FindString(text)
	return match
}
