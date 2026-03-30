package vboxmanage

import (
	"fmt"
	"strconv"
)

// ModifyVMOptions collects all flags for the VBoxManage modifyvm command.
// Pointer fields are used so that nil means "don't change this setting".
type ModifyVMOptions struct {
	Name               *string
	OSType             *string
	CPUs               *int
	Memory             *int    // MiB
	VRAM               *int    // MiB
	Firmware           *string // bios, efi, efi32, efi64
	GraphicsController *string // none, vboxvga, vmsvga, vboxsvga
	Clipboard          *string // disabled, hosttoguest, guesttohost, bidirectional
	DragAndDrop        *string // disabled, hosttoguest, guesttohost, bidirectional
	Description        *string
	Audio              *string // none, null, dsound, oss, alsa, pulse, coreaudio
	USB                *bool
	IOAPIC             *bool
	PAE                *bool
	ACPI               *bool
	HWVirtEx           *bool
	NestedPaging       *bool
	LargePages         *bool
	VTxVPID            *bool
	Accelerate3D       *bool
	RTC                *string // utc, local
	Boot1              *string
	Boot2              *string
	Boot3              *string
	Boot4              *string
}

// ModifyVM executes VBoxManage modifyvm to change VM settings.
// Only non-nil fields in opts will be applied as changes.
func ModifyVM(driver Driver, nameOrUUID string, opts ModifyVMOptions) error {
	args := []string{"modifyvm", nameOrUUID}

	appendString := func(flag string, val *string) {
		if val != nil {
			args = append(args, flag, *val)
		}
	}
	appendInt := func(flag string, val *int) {
		if val != nil {
			args = append(args, flag, strconv.Itoa(*val))
		}
	}
	appendBool := func(flag string, val *bool) {
		if val != nil {
			if *val {
				args = append(args, flag, "on")
			} else {
				args = append(args, flag, "off")
			}
		}
	}

	appendString("--name", opts.Name)
	appendString("--ostype", opts.OSType)
	appendInt("--cpus", opts.CPUs)
	appendInt("--memory", opts.Memory)
	appendInt("--vram", opts.VRAM)
	appendString("--firmware", opts.Firmware)
	appendString("--graphicscontroller", opts.GraphicsController)
	appendString("--clipboard-mode", opts.Clipboard)
	appendString("--draganddrop", opts.DragAndDrop)
	appendString("--description", opts.Description)
	appendString("--audio", opts.Audio)
	appendBool("--usb", opts.USB)
	appendBool("--ioapic", opts.IOAPIC)
	appendBool("--pae", opts.PAE)
	appendBool("--acpi", opts.ACPI)
	appendBool("--hwvirtex", opts.HWVirtEx)
	appendBool("--nestedpaging", opts.NestedPaging)
	appendBool("--largepages", opts.LargePages)
	appendBool("--vtxvpid", opts.VTxVPID)
	appendBool("--accelerate3d", opts.Accelerate3D)
	appendString("--rtcuseutc", opts.RTC)
	appendString("--boot1", opts.Boot1)
	appendString("--boot2", opts.Boot2)
	appendString("--boot3", opts.Boot3)
	appendString("--boot4", opts.Boot4)

	// Only run the command if there are actual modifications
	if len(args) == 2 {
		return nil
	}

	_, _, err := driver.Execute(args...)
	if err != nil {
		return fmt.Errorf("modifyvm %s: %w", nameOrUUID, err)
	}

	return nil
}
