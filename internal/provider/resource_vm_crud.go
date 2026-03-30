package provider

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	humanize "github.com/dustin/go-humanize"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	vbox "github.com/terra-farm/go-virtualbox"
)

func resourceVMExists(d *schema.ResourceData, meta any) (bool, error) {
	name := d.Get("name").(string)

	switch _, err := vbox.GetMachine(name); err {
	case nil:
		return true, nil
	case vbox.ErrMachineNotExist:
		return false, nil
	default:
		return false, fmt.Errorf("error when checking for existence of the VM: %w", err)
	}
}

func resourceVMCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	image := d.Get("image").(string)

	if addr, exists := d.GetOk("url"); exists {
		image = addr.(string)
	}

	u, err := url.Parse(image)
	if err != nil {
		return diag.Errorf("could not parse image URL: %v", err)
	}

	imagePath, err := fetchIfRemote(u)
	if err != nil {
		return diag.Errorf("unable to fetch remote image: %v", err)
	}

	/* Get gold folder and machine folder */
	usr, err := user.Current()
	if err != nil {
		return diag.Errorf("unable to get the current user: %v", err)
	}
	goldFolder := filepath.Join(usr.HomeDir, ".terraform/virtualbox/gold")
	machineFolder := filepath.Join(usr.HomeDir, ".terraform/virtualbox/machine")
	err = os.MkdirAll(goldFolder, 0740)
	if err != nil {
		return diag.Errorf("unable to create gold folder: %v", err)
	}
	err = os.MkdirAll(machineFolder, 0740)
	if err != nil {
		return diag.Errorf("unable to create machine folder: %v", err)
	}

	// Unpack gold image to gold folder
	imageOpMutex.Lock() // Sequentialize image unpacking to avoid conflicts
	goldFileName := filepath.Base(imagePath)
	goldName := strings.TrimSuffix(goldFileName, filepath.Ext(goldFileName))
	if filepath.Ext(goldName) == ".tar" {
		goldName = strings.TrimSuffix(goldName, ".tar")
	}

	goldPath := filepath.Join(goldFolder, goldName)
	if err = unpackImage(ctx, imagePath, goldPath); err != nil {
		imageOpMutex.Unlock()
		return diag.Errorf("failed to unpack image %s: %v", image, err)
	}
	imageOpMutex.Unlock()

	// Gather '*.vdi' and "*.vmdk" files from gold
	goldDisks, err := gatherDisks(goldPath)
	if err != nil {
		return diag.Errorf("unable to gather disks: %v", err)
	}

	// Create VM instance
	name := d.Get("name").(string)
	vm, err := vbox.CreateMachine(name, machineFolder)
	if err != nil {
		return diag.Errorf("can't create virtualbox VM %s: %v", name, err)
	}

	// Clone gold virtual disk files to VM folder
	for _, src := range goldDisks {
		filename := filepath.Base(src)

		target := filepath.Join(vm.BaseFolder, filename)

		if _, _, err := vbox.Run(ctx, "internalcommands", "sethduuid", src); err != nil {
			return diag.Errorf("unable to set UUID: %v", err)
		}

		imageOpMutex.Lock() // Sequentialize image cloning to improve disk performance
		err := vbox.CloneHD(src, target)
		imageOpMutex.Unlock()
		if err != nil {
			return diag.Errorf("failed to clone *.vdi and *.vmdk to VM folder: %v", err)
		}
	}

	// Attach virtual disks to VM
	vmDisks, err := gatherDisks(vm.BaseFolder)
	if err != nil {
		return diag.Errorf("unable to gather disks: %v", err)
	}

	if err := vm.AddStorageCtl("SATA", vbox.StorageController{
		SysBus:      vbox.SysBusSATA,
		Ports:       uint(len(vmDisks)) + 1,
		Chipset:     vbox.CtrlIntelAHCI,
		HostIOCache: true,
		Bootable:    true,
	}); err != nil {
		return diag.Errorf("can't create VirtualBox storage controller: %v", err)
	}

	for i, disk := range vmDisks {
		if err := vm.AttachStorage("SATA", vbox.StorageMedium{
			Port:      uint(i),
			Device:    0,
			DriveType: vbox.DriveHDD,
			Medium:    disk,
		}); err != nil {
			return diag.Errorf("failed to attach VirtualBox storage medium: %v", err)
		}
	}

	opticalDiskCount := d.Get("optical_disks.#").(int)
	opticalDisks := make([]string, 0, opticalDiskCount)

	for i := 0; i < opticalDiskCount; i++ {
		attr := fmt.Sprintf("optical_disks.%d", i)
		if opticalDiskImage, ok := d.Get(attr).(string); ok && attr != "" {
			opticalDisks = append(opticalDisks, opticalDiskImage)
		}
	}

	for i := 0; i < len(opticalDisks); i++ {
		opticalDiskImage := opticalDisks[i]
		fileName := filepath.Base(opticalDiskImage)

		target := filepath.Join(vm.BaseFolder, fileName)

		sourceFile, err := os.Open(opticalDiskImage)
		if err != nil {
			return diag.Errorf("failed to open source optical disk image: %v", err)
		}

		// make sure the file is closed when this function ends
		defer sourceFile.Close()

		targetFile, err := os.Create(target)
		if err != nil {
			return diag.Errorf("failed to create target optical disk image: %v", err)
		}

		// make sure the file is closed when this function ends
		defer targetFile.Close()

		if _, err := io.Copy(targetFile, sourceFile); err != nil {
			return diag.Errorf("copy optical disk image failed: %v", err)
		}

		// Explicitly sync & close the file now, so virtualbox can read it immediately, if we do not
		// do this, attaching the iso will fail.
		if err := targetFile.Sync(); err != nil {
			return diag.Errorf("sync target optical disk image to filesystem: %v", err)
		}

		if err := targetFile.Close(); err != nil {
			return diag.Errorf("close target optical disk image: %v", err)
		}

		if err := vm.AttachStorage("SATA", vbox.StorageMedium{
			Port:      uint(len(vmDisks) + i),
			Device:    0,
			DriveType: vbox.DriveDVD,
			Medium:    target,
		}); err != nil {
			return diag.Errorf("unable to attach VirtualBox storage medium: %v", err)
		}
	}

	// Create additional storage controllers
	ctlCount := d.Get("storage_controller.#").(int)
	for i := 0; i < ctlCount; i++ {
		prefix := fmt.Sprintf("storage_controller.%d.", i)
		ctlName := d.Get(prefix + "name").(string)
		ctlType := d.Get(prefix + "type").(string)

		args := []string{"storagectl", vm.UUID, "--name", ctlName, "--add", ctlType}

		if controller := d.Get(prefix + "controller").(string); controller != "" {
			args = append(args, "--controller", controller)
		}
		if portCount := d.Get(prefix + "port_count").(int); portCount > 0 {
			args = append(args, "--portcount", strconv.Itoa(portCount))
		}
		if d.Get(prefix + "host_io_cache").(bool) {
			args = append(args, "--hostiocache", "on")
		} else {
			args = append(args, "--hostiocache", "off")
		}
		if d.Get(prefix + "bootable").(bool) {
			args = append(args, "--bootable", "on")
		} else {
			args = append(args, "--bootable", "off")
		}

		if _, _, err := vbox.Run(ctx, args...); err != nil {
			return diag.Errorf("failed to add storage controller %s: %v", ctlName, err)
		}
	}

	// Attach additional disks
	daCount := d.Get("disk_attachment.#").(int)
	for i := 0; i < daCount; i++ {
		prefix := fmt.Sprintf("disk_attachment.%d.", i)
		ctlName := d.Get(prefix + "storage_controller").(string)
		port := d.Get(prefix + "port").(int)
		device := d.Get(prefix + "device").(int)
		driveType := d.Get(prefix + "drive_type").(string)
		medium := d.Get(prefix + "medium").(string)

		args := []string{"storageattach", vm.UUID,
			"--storagectl", ctlName,
			"--port", strconv.Itoa(port),
			"--device", strconv.Itoa(device),
			"--type", driveType,
			"--medium", medium,
		}

		if d.Get(prefix + "non_rotational").(bool) {
			args = append(args, "--nonrotational", "on")
		}
		if d.Get(prefix + "hot_pluggable").(bool) {
			args = append(args, "--hotpluggable", "on")
		}

		if _, _, err := vbox.Run(ctx, args...); err != nil {
			return diag.Errorf("failed to attach disk to %s port %d: %v", ctlName, port, err)
		}
	}

	// Setup VM general properties
	if err := tfToVbox(ctx, d, vm); err != nil {
		return diag.Errorf("unable to convert Terraform data to VM properties: %v", err)
	}
	if err := vm.Modify(); err != nil {
		return diag.Errorf("can't set up VM properties: %v", err)
	}

	// Apply firmware setting (not exposed by go-virtualbox)
	if firmware := d.Get("firmware").(string); firmware != "bios" {
		if err := applyFirmware(ctx, vm.UUID, firmware); err != nil {
			return diag.Errorf("unable to set firmware: %v", err)
		}
	}

	// Apply graphics controller (not exposed by go-virtualbox)
	if gc := d.Get("graphics_controller").(string); gc != "" {
		if err := applyGraphicsController(ctx, vm.UUID, gc); err != nil {
			return diag.Errorf("unable to set graphics controller: %v", err)
		}
	}

	// Apply per-NIC settings (port forwarding, promiscuous mode, etc.)
	if err := applyNICSettings(ctx, vm.UUID, d); err != nil {
		return diag.Errorf("unable to apply NIC settings: %v", err)
	}

	// Apply shared folders
	if err := applySharedFolders(ctx, vm.UUID, d); err != nil {
		return diag.Errorf("unable to apply shared folders: %v", err)
	}

	// Apply CPU execution cap
	if cap := d.Get("cpu_execution_cap").(int); cap < 100 {
		if err := applyCPUExecutionCap(ctx, vm.UUID, cap); err != nil {
			return diag.Errorf("unable to set CPU execution cap: %v", err)
		}
	}

	// Apply nested HW virtualization
	if d.Get("nested_hw_virt").(bool) {
		if err := applyNestedHWVirt(ctx, vm.UUID, true); err != nil {
			return diag.Errorf("unable to set nested HW virtualization: %v", err)
		}
	}

	// Apply USB controller
	if usb := d.Get("usb_controller").(string); usb != "" {
		if err := applyUSBController(ctx, vm.UUID, usb); err != nil {
			return diag.Errorf("unable to set USB controller: %v", err)
		}
	}

	// Apply clipboard and drag-and-drop
	if err := applyClipboardAndDragDrop(ctx, vm.UUID,
		d.Get("clipboard_mode").(string),
		d.Get("drag_and_drop").(string)); err != nil {
		return diag.Errorf("unable to set clipboard/drag-and-drop: %v", err)
	}

	// Apply chipset
	if err := applyChipset(ctx, vm.UUID, d.Get("chipset").(string)); err != nil {
		return diag.Errorf("unable to set chipset: %v", err)
	}

	// Apply serial ports
	if err := applySerialPorts(ctx, vm.UUID, d); err != nil {
		return diag.Errorf("unable to configure serial ports: %v", err)
	}

	// Run customize commands
	if customizations, ok := d.GetOk("customize"); ok {
		if err := executeCustomizations(ctx, vm.UUID, customizations.([]any)); err != nil {
			return diag.Errorf("customize commands failed: %v", err)
		}
	}

	// Start the VM
	if err := startVM(ctx, d, vm); err != nil {
		return diag.Errorf("unable to start VM: %v", err)
	}

	// Assign VM ID
	tflog.Debug(ctx, "resource ID", map[string]any{
		"uuid": vm.UUID,
	})
	d.SetId(vm.UUID)

	if err := waitUntilVMIsReady(ctx, d, vm, meta); err != nil {
		return diag.Errorf("failed to wait until VM is ready: %v", err)
	}

	// Errors here are already logged.
	return resourceVMRead(ctx, d, meta)
}

func setState(d *schema.ResourceData, state vbox.MachineState) error {
	var err error
	switch state {
	case vbox.Poweroff:
		err = d.Set("status", "poweroff")
	case vbox.Running:
		err = d.Set("status", "running")
	case vbox.Paused:
		err = d.Set("status", "paused")
	case vbox.Saved:
		err = d.Set("status", "saved")
	case vbox.Aborted:
		err = d.Set("status", "aborted")
	}
	if err != nil {
		return fmt.Errorf("unable to update VM state: %w", err)
	}
	return nil
}

func resourceVMRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	vm, err := vbox.GetMachine(d.Id())
	switch err {
	case nil:
		break
	case vbox.ErrMachineNotExist:
		// VM no longer exists.
		d.SetId("")
		return nil
	default:
		return diag.Errorf("unable to get machine: %v", err)
	}

	// if vm.State != vbox.Running {
	//	setState(d, vm.State)
	//	return nil
	// }

	err = setState(d, vm.State)
	if err != nil {
		return diag.Errorf("can't set state: %v", err)
	}
	err = d.Set("name", vm.Name)
	if err != nil {
		return diag.Errorf("can't set name: %v", err)
	}
	err = d.Set("cpus", vm.CPUs)
	if err != nil {
		return diag.Errorf("can't set cpus: %v", err)
	}
	bytes := uint64(vm.Memory) * humanize.MiByte
	repr := humanize.IBytes(bytes)
	err = d.Set("memory", strings.ToLower(repr))
	if err != nil {
		return diag.Errorf("can't set memory: %v", err)
	}

	if err = netVboxToTf(vm, d); err != nil {
		return diag.Errorf("can't convert vbox network to terraform data: %v", err)
	}

	/* Set connection info to first non NAT IPv4 address */
	for i, nic := range vm.NICs {
		if nic.Network == vbox.NICNetNAT {
			continue
		}
		availKey := fmt.Sprintf("network_adapter.%d.ipv4_address_available", i)
		if d.Get(availKey).(string) != "yes" {
			continue
		}
		ipv4Key := fmt.Sprintf("network_adapter.%d.ipv4_address", i)
		ipv4 := d.Get(ipv4Key).(string)
		if ipv4 == "" {
			continue
		}
		d.SetConnInfo(map[string]string{
			"type": "ssh",
			"host": ipv4,
		})
		break
	}

	err = d.Set("boot_order", vm.BootOrder)
	if err != nil {
		return diag.Errorf("can't set boot_order: %v", err)
	}

	return nil
}

func resourceVMUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	// TODO: allow partial updates

	vm, err := vbox.GetMachine(d.Id())
	if err != nil {
		return diag.Errorf("unable to get machine %s: %v", d.Id(), err)
	}

	if err := vm.Poweroff(); err != nil {
		return diag.Errorf("unable to poweroff machine %s: %v", d.Id(), err)
	}

	// Modify VM
	if err := tfToVbox(ctx, d, vm); err != nil {
		return diag.Errorf("can't convert terraform config to virtual machine: %v", err)
	}
	if err := vm.Modify(); err != nil {
		return diag.Errorf("unable to modify the vm: %v", err)
	}

	// Apply firmware setting
	if firmware := d.Get("firmware").(string); firmware != "bios" {
		if err := applyFirmware(ctx, vm.UUID, firmware); err != nil {
			return diag.Errorf("unable to set firmware: %v", err)
		}
	}

	// Apply graphics controller
	if gc := d.Get("graphics_controller").(string); gc != "" {
		if err := applyGraphicsController(ctx, vm.UUID, gc); err != nil {
			return diag.Errorf("unable to set graphics controller: %v", err)
		}
	}

	// Apply per-NIC settings (port forwarding, promiscuous mode, etc.)
	if err := applyNICSettings(ctx, vm.UUID, d); err != nil {
		return diag.Errorf("unable to apply NIC settings: %v", err)
	}

	// Update shared folders (remove old, add new)
	if err := removeSharedFolders(ctx, vm.UUID, d); err != nil {
		return diag.Errorf("unable to remove shared folders: %v", err)
	}
	if err := applySharedFolders(ctx, vm.UUID, d); err != nil {
		return diag.Errorf("unable to apply shared folders: %v", err)
	}

	// Apply CPU execution cap
	if cap := d.Get("cpu_execution_cap").(int); cap < 100 {
		if err := applyCPUExecutionCap(ctx, vm.UUID, cap); err != nil {
			return diag.Errorf("unable to set CPU execution cap: %v", err)
		}
	}

	// Apply nested HW virtualization
	if err := applyNestedHWVirt(ctx, vm.UUID, d.Get("nested_hw_virt").(bool)); err != nil {
		return diag.Errorf("unable to set nested HW virtualization: %v", err)
	}

	// Apply USB controller
	if usb := d.Get("usb_controller").(string); usb != "" {
		if err := applyUSBController(ctx, vm.UUID, usb); err != nil {
			return diag.Errorf("unable to set USB controller: %v", err)
		}
	}

	// Apply clipboard and drag-and-drop
	if err := applyClipboardAndDragDrop(ctx, vm.UUID,
		d.Get("clipboard_mode").(string),
		d.Get("drag_and_drop").(string)); err != nil {
		return diag.Errorf("unable to set clipboard/drag-and-drop: %v", err)
	}

	// Apply chipset
	if err := applyChipset(ctx, vm.UUID, d.Get("chipset").(string)); err != nil {
		return diag.Errorf("unable to set chipset: %v", err)
	}

	// Apply serial ports
	if err := applySerialPorts(ctx, vm.UUID, d); err != nil {
		return diag.Errorf("unable to configure serial ports: %v", err)
	}

	// Run customize commands
	if customizations, ok := d.GetOk("customize"); ok {
		if err := executeCustomizations(ctx, vm.UUID, customizations.([]any)); err != nil {
			return diag.Errorf("customize commands failed: %v", err)
		}
	}

	if err := powerOnAndWait(ctx, d, vm, meta); err != nil {
		return diag.Errorf("unable to power on and wait for VM: %v", err)
	}

	// Errors are already logged
	return resourceVMRead(ctx, d, meta)
}

func resourceVMDelete(d *schema.ResourceData, meta any) error {
	vm, err := vbox.GetMachine(d.Id())
	if err != nil {
		return fmt.Errorf("unable to get machine for deletion: %w", err)
	}
	if err := vm.Delete(); err != nil {
		return fmt.Errorf("unable to remove the VM: %w", err)
	}
	return nil
}
