package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// applyUserData sets cloud-init user data via VirtualBox guest properties.
// Cloud-init in the guest can read these properties to configure the VM.
// This uses the GuestInfo mechanism that cloud-init's "OVF" and "GuestInfo"
// datasources can consume.
func applyUserData(ctx context.Context, vmUUID string, userData string) error {
	if userData == "" {
		return nil
	}

	tflog.Debug(ctx, "setting cloud-init user data via guest properties", map[string]any{
		"vm": vmUUID,
	})

	// Set user-data via guest property (cloud-init GuestInfo datasource)
	if _, _, err := vboxRun(ctx, "guestproperty", "set", vmUUID,
		"/VirtualBox/GuestInfo/userdata", userData); err != nil {
		return fmt.Errorf("failed to set user data guest property: %w", err)
	}

	// Also set via extradata for cloud-init OVF datasource compatibility
	if _, _, err := vboxRun(ctx, "setextradata", vmUUID,
		"VBoxInternal/Devices/VMMDev/0/Config/GetHostTimeDisabled", "1"); err != nil {
		// Non-fatal, just log
		tflog.Warn(ctx, "failed to set extra data", map[string]any{"error": err.Error()})
	}

	return nil
}
