package vboxmanage

import (
	"fmt"
	"strconv"
)

// StorageAttachOptions defines options for attaching storage media to a VM.
type StorageAttachOptions struct {
	StorageCtl    string // Name of the storage controller
	Port          int    // Port number on the controller
	Device        int    // Device number on the port
	Type          string // hdd, dvddrive, fdd
	Medium        string // File path, "none", "emptydrive", "additions"
	NonRotational bool   // Marks the medium as an SSD
	HotPluggable  bool   // Allows hot-plugging the device
}

// StorageAttach attaches a storage medium to a VM using VBoxManage storageattach.
func StorageAttach(driver Driver, vmNameOrUUID string, opts StorageAttachOptions) error {
	args := []string{
		"storageattach", vmNameOrUUID,
		"--storagectl", opts.StorageCtl,
		"--port", strconv.Itoa(opts.Port),
		"--device", strconv.Itoa(opts.Device),
	}

	if opts.Type != "" {
		args = append(args, "--type", opts.Type)
	}
	if opts.Medium != "" {
		args = append(args, "--medium", opts.Medium)
	}
	if opts.NonRotational {
		args = append(args, "--nonrotational", "on")
	}
	if opts.HotPluggable {
		args = append(args, "--hotpluggable", "on")
	}

	_, _, err := driver.Execute(args...)
	if err != nil {
		return fmt.Errorf("storageattach %s (ctl=%s port=%d device=%d): %w",
			vmNameOrUUID, opts.StorageCtl, opts.Port, opts.Device, err)
	}
	return nil
}

// StorageDetach detaches a storage medium from a VM by setting the medium to "none".
func StorageDetach(driver Driver, vmNameOrUUID string, ctlName string, port int, device int) error {
	_, _, err := driver.Execute(
		"storageattach", vmNameOrUUID,
		"--storagectl", ctlName,
		"--port", strconv.Itoa(port),
		"--device", strconv.Itoa(device),
		"--medium", "none",
	)
	if err != nil {
		return fmt.Errorf("storagedetach %s (ctl=%s port=%d device=%d): %w",
			vmNameOrUUID, ctlName, port, device, err)
	}
	return nil
}
