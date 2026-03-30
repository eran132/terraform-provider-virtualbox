package vboxmanage

import (
	"fmt"
	"strconv"
)

// StorageCtlOptions defines options for adding a storage controller to a VM.
type StorageCtlOptions struct {
	Name        string // Required: name of the storage controller
	Add         string // ide, sata, scsi, floppy, sas, pcie, virtio
	Controller  string // IntelAHCI, PIIX4, LSILogic, BusLogic, NVMe, VirtIO
	PortCount   int
	HostIOCache bool
	Bootable    bool
}

// AddStorageCtl adds a storage controller to a VM using VBoxManage storagectl.
func AddStorageCtl(driver Driver, vmNameOrUUID string, opts StorageCtlOptions) error {
	args := []string{"storagectl", vmNameOrUUID, "--name", opts.Name}

	if opts.Add != "" {
		args = append(args, "--add", opts.Add)
	}
	if opts.Controller != "" {
		args = append(args, "--controller", opts.Controller)
	}
	if opts.PortCount > 0 {
		args = append(args, "--portcount", strconv.Itoa(opts.PortCount))
	}

	if opts.HostIOCache {
		args = append(args, "--hostiocache", "on")
	} else {
		args = append(args, "--hostiocache", "off")
	}

	if opts.Bootable {
		args = append(args, "--bootable", "on")
	} else {
		args = append(args, "--bootable", "off")
	}

	_, _, err := driver.Execute(args...)
	if err != nil {
		return fmt.Errorf("storagectl add %s on %s: %w", opts.Name, vmNameOrUUID, err)
	}
	return nil
}

// RemoveStorageCtl removes a storage controller from a VM.
func RemoveStorageCtl(driver Driver, vmNameOrUUID string, ctlName string) error {
	_, _, err := driver.Execute("storagectl", vmNameOrUUID, "--name", ctlName, "--remove")
	if err != nil {
		return fmt.Errorf("storagectl remove %s from %s: %w", ctlName, vmNameOrUUID, err)
	}
	return nil
}
