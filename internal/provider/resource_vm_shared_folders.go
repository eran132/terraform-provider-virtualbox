package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// applySharedFolders configures shared folders on the VM.
// Must be called after vm.Modify() and before vm.Start().
func applySharedFolders(ctx context.Context, vmUUID string, d *schema.ResourceData) error {
	sfCount := d.Get("shared_folder.#").(int)
	for i := 0; i < sfCount; i++ {
		prefix := fmt.Sprintf("shared_folder.%d.", i)
		name := d.Get(prefix + "name").(string)
		hostPath := d.Get(prefix + "host_path").(string)

		args := []string{"sharedfolder", "add", vmUUID,
			"--name", name,
			"--hostpath", hostPath,
		}

		if d.Get(prefix + "read_only").(bool) {
			args = append(args, "--readonly")
		}
		if d.Get(prefix + "auto_mount").(bool) {
			args = append(args, "--automount")
		}
		if mountPoint := d.Get(prefix + "mount_point").(string); mountPoint != "" {
			args = append(args, "--auto-mount-point", mountPoint)
		}

		if _, _, err := vboxRun(ctx, args...); err != nil {
			return fmt.Errorf("failed to add shared folder %q: %w", name, err)
		}
	}
	return nil
}

// removeSharedFolders removes all shared folders from a VM.
func removeSharedFolders(ctx context.Context, vmUUID string, d *schema.ResourceData) error {
	sfCount := d.Get("shared_folder.#").(int)
	for i := 0; i < sfCount; i++ {
		prefix := fmt.Sprintf("shared_folder.%d.", i)
		name := d.Get(prefix + "name").(string)
		// Ignore errors since folder may not exist
		vboxRun(ctx, "sharedfolder", "remove", vmUUID, "--name", name) //nolint:errcheck
	}
	return nil
}

// applyCPUExecutionCap sets the CPU execution cap via VBoxManage.
func applyCPUExecutionCap(ctx context.Context, vmUUID string, cap int) error {
	if cap >= 100 {
		return nil // 100% is default, no action needed
	}
	_, _, err := vboxRun(ctx, "modifyvm", vmUUID, "--cpuexecutioncap", fmt.Sprintf("%d", cap))
	if err != nil {
		return fmt.Errorf("failed to set CPU execution cap to %d%%: %w", cap, err)
	}
	return nil
}

// applyNestedHWVirt enables/disables nested hardware virtualization.
func applyNestedHWVirt(ctx context.Context, vmUUID string, enabled bool) error {
	val := "off"
	if enabled {
		val = "on"
	}
	_, _, err := vboxRun(ctx, "modifyvm", vmUUID, "--nested-hw-virt", val)
	if err != nil {
		return fmt.Errorf("failed to set nested HW virtualization: %w", err)
	}
	return nil
}
