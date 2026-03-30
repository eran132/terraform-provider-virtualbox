package vboxmanage

import (
	"fmt"
	"strings"
)

// GetGuestProperty retrieves a guest property value from a VM using
// VBoxManage guestproperty get.
func GetGuestProperty(driver Driver, vmUUID string, property string) (string, error) {
	stdout, _, err := driver.Execute("guestproperty", "get", vmUUID, property)
	if err != nil {
		return "", fmt.Errorf("guestproperty get %s %s: %w", vmUUID, property, err)
	}

	stdout = strings.TrimSpace(stdout)

	// VBoxManage returns "No value set!" when the property doesn't exist
	if stdout == "No value set!" {
		return "", nil
	}

	// Output format: "Value: <value>"
	if strings.HasPrefix(stdout, "Value: ") {
		return strings.TrimPrefix(stdout, "Value: "), nil
	}

	return stdout, nil
}

// SetGuestProperty sets a guest property on a VM using
// VBoxManage guestproperty set.
func SetGuestProperty(driver Driver, vmUUID string, property string, value string) error {
	_, _, err := driver.Execute("guestproperty", "set", vmUUID, property, value)
	if err != nil {
		return fmt.Errorf("guestproperty set %s %s: %w", vmUUID, property, err)
	}
	return nil
}
