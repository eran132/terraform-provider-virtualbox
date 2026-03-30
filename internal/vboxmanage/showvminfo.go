package vboxmanage

import (
	"fmt"
	"strconv"
	"strings"
)

// VMInfo represents the parsed output of VBoxManage showvminfo --machinereadable.
type VMInfo struct {
	UUID        string
	Name        string
	OSType      string
	CPUs        int
	Memory      int // MiB
	VRAM        int // MiB
	State       string
	Firmware    string
	NICs        []NICInfo
	StorageCtls []StorageCtlInfo
}

// NICInfo represents a network interface card configuration for a VM.
type NICInfo struct {
	Index          int
	Enabled        bool
	Type           string // nat, bridged, hostonly, internal, generic, natnetwork
	Device         string // e.g. "Intel PRO/1000 MT Desktop (82540EM)"
	MACAddress     string
	HostInterface  string
	CableConnected bool
}

// StorageCtlInfo represents a storage controller attached to a VM.
type StorageCtlInfo struct {
	Name       string
	Type       string // IDE, SATA, SCSI, Floppy, SAS, PCIe, VirtIO
	Instance   int
	PortCount  int
	Bootable   bool
	Controller string
}

// ShowVMInfo retrieves and parses detailed VM information using
// VBoxManage showvminfo --machinereadable.
func ShowVMInfo(driver Driver, nameOrUUID string) (*VMInfo, error) {
	stdout, _, err := driver.Execute("showvminfo", nameOrUUID, "--machinereadable")
	if err != nil {
		return nil, fmt.Errorf("showvminfo %s: %w", nameOrUUID, err)
	}

	return parseVMInfo(stdout)
}

// parseVMInfo parses the machine-readable key=value output from showvminfo.
func parseVMInfo(output string) (*VMInfo, error) {
	info := &VMInfo{}
	kvs := parseMachineReadable(output)

	info.UUID = kvs["UUID"]
	info.Name = kvs["name"]
	info.OSType = kvs["ostype"]
	info.State = kvs["VMState"]
	info.Firmware = kvs["firmware"]

	if v, ok := kvs["cpus"]; ok {
		info.CPUs, _ = strconv.Atoi(v)
	}
	if v, ok := kvs["memory"]; ok {
		info.Memory, _ = strconv.Atoi(v)
	}
	if v, ok := kvs["vram"]; ok {
		info.VRAM, _ = strconv.Atoi(v)
	}

	// Parse NICs (VirtualBox supports up to 8 NICs, indexed 1-8)
	for i := 1; i <= 8; i++ {
		prefix := fmt.Sprintf("nic%d", i)
		nicType, exists := kvs[prefix]
		if !exists || nicType == "none" {
			continue
		}
		nic := NICInfo{
			Index:   i,
			Enabled: true,
			Type:    nicType,
		}
		nic.Device = kvs[fmt.Sprintf("nictype%d", i)]
		nic.MACAddress = kvs[fmt.Sprintf("macaddress%d", i)]
		nic.HostInterface = kvs[fmt.Sprintf("hostonlyadapter%d", i)]
		if nic.HostInterface == "" {
			nic.HostInterface = kvs[fmt.Sprintf("bridgeadapter%d", i)]
		}
		cable := kvs[fmt.Sprintf("cableconnected%d", i)]
		nic.CableConnected = cable == "on"
		info.NICs = append(info.NICs, nic)
	}

	// Parse storage controllers
	for i := 0; i < 16; i++ {
		nameKey := fmt.Sprintf("storagecontrollername%d", i)
		ctlName, exists := kvs[nameKey]
		if !exists {
			break
		}
		ctl := StorageCtlInfo{
			Name: ctlName,
		}
		ctl.Type = kvs[fmt.Sprintf("storagecontrollertype%d", i)]
		ctl.Controller = kvs[fmt.Sprintf("storagecontrollertype%d", i)]
		if v, ok := kvs[fmt.Sprintf("storagecontrollerinstance%d", i)]; ok {
			ctl.Instance, _ = strconv.Atoi(v)
		}
		if v, ok := kvs[fmt.Sprintf("storagecontrollerportcount%d", i)]; ok {
			ctl.PortCount, _ = strconv.Atoi(v)
		}
		bootable := kvs[fmt.Sprintf("storagecontrollerbootable%d", i)]
		ctl.Bootable = bootable == "on"
		info.StorageCtls = append(info.StorageCtls, ctl)
	}

	return info, nil
}

// parseMachineReadable parses VBoxManage --machinereadable key=value output
// into a map. It handles quoted and unquoted values.
func parseMachineReadable(output string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}

		key := line[:idx]
		value := line[idx+1:]

		// Remove surrounding quotes from both key and value
		key = strings.Trim(key, "\"")
		value = strings.Trim(value, "\"")

		result[key] = value
	}

	return result
}
