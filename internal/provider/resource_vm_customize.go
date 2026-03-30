package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	vbox "github.com/terra-farm/go-virtualbox"
)

// dangerousCommands lists VBoxManage subcommands that should not be allowed
// in the customize escape hatch as they could corrupt provider-managed state.
var dangerousCommands = map[string]bool{
	"unregistervm": true,
	"closemedium":  true,
	"startvm":      true,
}

// executeCustomizations runs arbitrary VBoxManage commands from the customize
// block. The special token ":id" in any argument is replaced with the VM UUID.
func executeCustomizations(ctx context.Context, vmUUID string, customizations []any) error {
	for i, item := range customizations {
		cmdList, ok := item.([]any)
		if !ok {
			return fmt.Errorf("customize[%d]: expected a list of strings, got %T", i, item)
		}

		if len(cmdList) == 0 {
			continue
		}

		args := make([]string, 0, len(cmdList))
		for _, arg := range cmdList {
			s, ok := arg.(string)
			if !ok {
				return fmt.Errorf("customize[%d]: argument must be a string, got %T", i, arg)
			}
			// Replace :id placeholder with VM UUID
			s = strings.ReplaceAll(s, ":id", vmUUID)
			args = append(args, s)
		}

		// Validate: block dangerous commands
		if len(args) > 0 {
			subcmd := strings.ToLower(args[0])
			if dangerousCommands[subcmd] {
				return fmt.Errorf("customize[%d]: command %q is not allowed in customize blocks", i, args[0])
			}
		}

		tflog.Info(ctx, "running VBoxManage customization", map[string]any{
			"index": i,
			"args":  strings.Join(args, " "),
		})

		if _, _, err := vbox.Run(ctx, args...); err != nil {
			return fmt.Errorf("customize[%d] VBoxManage %s failed: %w", i, strings.Join(args, " "), err)
		}
	}

	return nil
}

// applyFirmware sets the firmware type via VBoxManage modifyvm since
// go-virtualbox doesn't expose this property.
func applyFirmware(ctx context.Context, vmUUID string, firmware string) error {
	if firmware == "" || firmware == "bios" {
		// "bios" is the VirtualBox default, no action needed unless explicitly changing
		return nil
	}
	_, _, err := vbox.Run(ctx, "modifyvm", vmUUID, "--firmware", firmware)
	if err != nil {
		return fmt.Errorf("failed to set firmware to %s: %w", firmware, err)
	}
	return nil
}

// applyGraphicsController sets the graphics controller via VBoxManage modifyvm
// since go-virtualbox doesn't expose this property.
func applyGraphicsController(ctx context.Context, vmUUID string, controller string) error {
	if controller == "" {
		return nil
	}
	_, _, err := vbox.Run(ctx, "modifyvm", vmUUID, "--graphicscontroller", controller)
	if err != nil {
		return fmt.Errorf("failed to set graphics controller to %s: %w", controller, err)
	}
	return nil
}

// applyUSBController configures the USB controller type.
func applyUSBController(ctx context.Context, vmUUID string, controller string) error {
	if controller == "" {
		return nil
	}
	flag := "--usb"
	switch controller {
	case "ohci":
		flag = "--usbohci"
	case "ehci":
		flag = "--usbehci"
	case "xhci":
		flag = "--usbxhci"
	}
	_, _, err := vbox.Run(ctx, "modifyvm", vmUUID, flag, "on")
	if err != nil {
		return fmt.Errorf("failed to set USB controller to %s: %w", controller, err)
	}
	return nil
}

// applyClipboardAndDragDrop sets clipboard and drag-and-drop modes.
func applyClipboardAndDragDrop(ctx context.Context, vmUUID string, clipboard string, dragDrop string) error {
	if clipboard != "disabled" {
		if _, _, err := vbox.Run(ctx, "modifyvm", vmUUID, "--clipboard-mode", clipboard); err != nil {
			return fmt.Errorf("failed to set clipboard mode: %w", err)
		}
	}
	if dragDrop != "disabled" {
		if _, _, err := vbox.Run(ctx, "modifyvm", vmUUID, "--draganddrop", dragDrop); err != nil {
			return fmt.Errorf("failed to set drag and drop mode: %w", err)
		}
	}
	return nil
}

// applyChipset sets the chipset type.
func applyChipset(ctx context.Context, vmUUID string, chipset string) error {
	if chipset == "" || chipset == "piix3" {
		return nil // piix3 is default
	}
	_, _, err := vbox.Run(ctx, "modifyvm", vmUUID, "--chipset", chipset)
	if err != nil {
		return fmt.Errorf("failed to set chipset to %s: %w", chipset, err)
	}
	return nil
}

// applySerialPorts configures serial ports on the VM.
func applySerialPorts(ctx context.Context, vmUUID string, d *schema.ResourceData) error {
	spCount := d.Get("serial_port.#").(int)
	for i := 0; i < spCount; i++ {
		prefix := fmt.Sprintf("serial_port.%d.", i)
		slot := d.Get(prefix + "slot").(int)
		mode := d.Get(prefix + "mode").(string)
		path := d.Get(prefix + "path").(string)

		// Enable the UART at default I/O base and IRQ for each slot
		ioBase := []string{"0x3F8", "0x2F8", "0x3E8", "0x2E8"}
		irq := []string{"4", "3", "4", "3"}
		if slot >= 0 && slot < 4 {
			uartFlag := fmt.Sprintf("--uart%d", slot+1)
			if _, _, err := vbox.Run(ctx, "modifyvm", vmUUID, uartFlag, ioBase[slot], irq[slot]); err != nil {
				return fmt.Errorf("failed to enable UART slot %d: %w", slot, err)
			}

			modeFlag := fmt.Sprintf("--uartmode%d", slot+1)
			args := []string{"modifyvm", vmUUID, modeFlag, mode}
			if path != "" {
				args = append(args, path)
			}
			if _, _, err := vbox.Run(ctx, args...); err != nil {
				return fmt.Errorf("failed to set serial port %d mode: %w", slot, err)
			}
		}
	}
	return nil
}
