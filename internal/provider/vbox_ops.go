package provider

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// vboxManagePath caches the detected VBoxManage path.
var vboxManagePath string

// detectVBoxManage finds the VBoxManage binary.
func detectVBoxManage() (string, error) {
	if vboxManagePath != "" {
		return vboxManagePath, nil
	}
	path, err := exec.LookPath("VBoxManage")
	if err == nil {
		vboxManagePath = path
		return path, nil
	}
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		candidates = []string{
			`C:\Program Files\Oracle\VirtualBox\VBoxManage.exe`,
			`C:\Program Files (x86)\Oracle\VirtualBox\VBoxManage.exe`,
		}
	case "darwin":
		candidates = []string{
			"/usr/local/bin/VBoxManage",
			"/Applications/VirtualBox.app/Contents/MacOS/VBoxManage",
		}
	default:
		candidates = []string{"/usr/bin/VBoxManage", "/usr/local/bin/VBoxManage"}
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			vboxManagePath = c
			return c, nil
		}
	}
	return "", fmt.Errorf("VBoxManage binary not found")
}

// vboxRun executes a VBoxManage command and returns stdout, stderr.
func vboxRun(ctx context.Context, args ...string) (string, string, error) {
	path, err := detectVBoxManage()
	if err != nil {
		return "", "", err
	}
	cmd := exec.CommandContext(ctx, path, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return stdout.String(), stderr.String(), err
}

// ErrMachineNotExist is returned when a VM is not found.
var ErrMachineNotExist = fmt.Errorf("machine does not exist")

// getMachine retrieves a VM by name or UUID using showvminfo --machinereadable.
func getMachine(nameOrUUID string) (*Machine, error) {
	stdout, _, err := vboxRun(context.Background(), "showvminfo", nameOrUUID, "--machinereadable")
	if err != nil {
		if strings.Contains(fmt.Sprint(err), "VBOX_E_OBJECT_NOT_FOUND") {
			return nil, ErrMachineNotExist
		}
		// Also check stderr/stdout content for "not found" patterns
		if strings.Contains(stdout, "VBOX_E_OBJECT_NOT_FOUND") {
			return nil, ErrMachineNotExist
		}
		return nil, fmt.Errorf("failed to get machine %s: %w", nameOrUUID, err)
	}
	return parseMachineInfo(stdout)
}

// parseMachineInfo parses showvminfo --machinereadable output.
func parseMachineInfo(output string) (*Machine, error) {
	m := &Machine{
		BootOrder: []string{"none", "none", "none", "none"},
	}
	props := make(map[string]string)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := strings.Trim(parts[1], "\"")
		props[key] = value
	}

	m.UUID = props["UUID"]
	m.Name = props["name"]
	m.OSType = props["ostype"]
	if v, err := strconv.Atoi(props["cpus"]); err == nil {
		m.CPUs = uint(v)
	}
	if v, err := strconv.Atoi(props["memory"]); err == nil {
		m.Memory = uint(v)
	}
	if v, err := strconv.Atoi(props["vram"]); err == nil {
		m.VRAM = uint(v)
	}

	// Parse state
	switch props["VMState"] {
	case "running":
		m.State = MachineStateRunning
	case "poweroff", "powered off":
		m.State = MachineStatePoweroff
	case "paused":
		m.State = MachineStatePaused
	case "saved":
		m.State = MachineStateSaved
	case "aborted":
		m.State = MachineStateAborted
	default:
		m.State = MachineStatePoweroff
	}

	// Parse NICs (up to 8)
	for i := 1; i <= 8; i++ {
		nicKey := fmt.Sprintf("nic%d", i)
		nicVal := props[nicKey]
		if nicVal == "" || nicVal == "none" {
			continue
		}
		nic := NIC{}
		switch nicVal {
		case "nat":
			nic.Network = NICNetNAT
		case "bridged":
			nic.Network = NICNetBridged
		case "hostonly":
			nic.Network = NICNetHostonly
		case "intnet":
			nic.Network = NICNetInternal
		case "generic":
			nic.Network = NICNetGeneric
		}
		nic.MacAddr = props[fmt.Sprintf("macaddress%d", i)]
		nic.HostInterface = props[fmt.Sprintf("hostonlyadapter%d", i)]
		if nic.HostInterface == "" {
			nic.HostInterface = props[fmt.Sprintf("bridgeadapter%d", i)]
		}
		m.NICs = append(m.NICs, nic)
	}

	// Parse boot order
	for i := 1; i <= 4; i++ {
		if v, ok := props[fmt.Sprintf("boot%d", i)]; ok && v != "" {
			m.BootOrder[i-1] = v
		}
	}

	// Parse CfgFile to get base folder
	if cfgFile, ok := props["CfgFile"]; ok {
		m.BaseFolder = filepath.Dir(cfgFile)
	}

	return m, nil
}

// createMachine creates a new VM via VBoxManage createvm.
func createMachine(ctx context.Context, name string, baseFolder string) (*Machine, error) {
	args := []string{"createvm", "--name", name, "--basefolder", baseFolder, "--register"}
	stdout, _, err := vboxRun(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create machine %s: %w", name, err)
	}

	// Parse UUID from output
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "UUID:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				uuid := strings.TrimSpace(parts[1])
				return getMachine(uuid)
			}
		}
	}

	// Fall back to getting by name
	return getMachine(name)
}

// cloneHD clones a virtual disk.
func cloneHD(ctx context.Context, src, target string) error {
	_, _, err := vboxRun(ctx, "clonemedium", "disk", src, target)
	return err
}

// modifyVM applies Machine properties via VBoxManage modifyvm.
func modifyVM(ctx context.Context, m *Machine) error {
	args := []string{"modifyvm", m.UUID,
		"--ostype", m.OSType,
		"--cpus", strconv.Itoa(int(m.CPUs)),
		"--memory", strconv.Itoa(int(m.Memory)),
		"--vram", strconv.Itoa(int(m.VRAM)),
	}

	// Hardware flags
	boolFlag := func(flag uint, name string) {
		if m.Flag&flag != 0 {
			args = append(args, name, "on")
		} else {
			args = append(args, name, "off")
		}
	}
	boolFlag(FlagACPI, "--acpi")
	boolFlag(FlagIOAPIC, "--ioapic")
	boolFlag(FlagRTCUSEUTC, "--rtcuseutc")
	boolFlag(FlagPAE, "--pae")
	boolFlag(FlagHWVIRTEX, "--hwvirtex")
	boolFlag(FlagNESTEDPAGING, "--nestedpaging")
	boolFlag(FlagLARGEPAGES, "--largepages")
	boolFlag(FlagLONGMODE, "--longmode")
	boolFlag(FlagVTXVPID, "--vtxvpid")
	boolFlag(FlagVTXUX, "--vtxux")

	// NICs
	for i, nic := range m.NICs {
		idx := strconv.Itoa(i + 1)
		args = append(args, "--nic"+idx, string(nic.Network))
		if nic.Hardware != "" {
			args = append(args, "--nictype"+idx, string(nic.Hardware))
		}
		if nic.HostInterface != "" {
			switch nic.Network {
			case NICNetHostonly:
				args = append(args, "--hostonlyadapter"+idx, nic.HostInterface)
			case NICNetBridged:
				args = append(args, "--bridgeadapter"+idx, nic.HostInterface)
			}
		}
	}

	// Boot order
	for i, dev := range m.BootOrder {
		args = append(args, fmt.Sprintf("--boot%d", i+1), dev)
	}

	_, _, err := vboxRun(ctx, args...)
	return err
}

// addStorageCtl adds a storage controller.
func addStorageCtl(ctx context.Context, vmUUID string, name string, busType string, chipset string, ports int, hostIOCache bool, bootable bool) error {
	args := []string{"storagectl", vmUUID, "--name", name, "--add", busType}
	if chipset != "" {
		args = append(args, "--controller", chipset)
	}
	if ports > 0 {
		args = append(args, "--portcount", strconv.Itoa(ports))
	}
	if hostIOCache {
		args = append(args, "--hostiocache", "on")
	}
	if bootable {
		args = append(args, "--bootable", "on")
	}
	_, _, err := vboxRun(ctx, args...)
	return err
}

// attachStorage attaches a medium to a storage controller.
func attachStorage(ctx context.Context, vmUUID string, ctlName string, port int, device int, driveType string, medium string) error {
	args := []string{"storageattach", vmUUID,
		"--storagectl", ctlName,
		"--port", strconv.Itoa(port),
		"--device", strconv.Itoa(device),
		"--type", driveType,
		"--medium", medium,
	}
	_, _, err := vboxRun(ctx, args...)
	return err
}

// getGuestProperty reads a guest property.
func getGuestProperty(vmUUID string, prop string) (string, error) {
	stdout, _, err := vboxRun(context.Background(), "guestproperty", "get", vmUUID, prop)
	if err != nil {
		return "", err
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "No value set!" || stdout == "" {
		return "", nil
	}
	// Parse "Value: <value>"
	if strings.HasPrefix(stdout, "Value: ") {
		return strings.TrimPrefix(stdout, "Value: "), nil
	}
	return stdout, nil
}

// startVMHeadless starts a VM in headless mode.
func startVMHeadless(ctx context.Context, uuid string) error {
	_, _, err := vboxRun(ctx, "startvm", uuid, "--type", "headless")
	return err
}

// startVMGUI starts a VM with GUI.
func startVMGUI(ctx context.Context, uuid string) error {
	_, _, err := vboxRun(ctx, "startvm", uuid, "--type", "gui")
	return err
}
