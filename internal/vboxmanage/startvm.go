package vboxmanage

import (
	"fmt"
)

// StartVM starts a VM with the specified display mode using VBoxManage startvm.
// vmType can be "headless", "gui", "sdl", or "separate".
func StartVM(driver Driver, nameOrUUID string, vmType string) error {
	args := []string{"startvm", nameOrUUID}

	if vmType != "" {
		args = append(args, "--type", vmType)
	}

	_, _, err := driver.Execute(args...)
	if err != nil {
		return fmt.Errorf("startvm %s (type=%s): %w", nameOrUUID, vmType, err)
	}
	return nil
}

// ControlVM sends a control command to a running VM using VBoxManage controlvm.
// Supported actions: "pause", "resume", "reset", "poweroff", "savestate", "acpipowerbutton".
func ControlVM(driver Driver, nameOrUUID string, action string) error {
	_, _, err := driver.Execute("controlvm", nameOrUUID, action)
	if err != nil {
		return fmt.Errorf("controlvm %s %s: %w", nameOrUUID, action, err)
	}
	return nil
}
