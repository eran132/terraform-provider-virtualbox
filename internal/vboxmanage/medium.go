package vboxmanage

import (
	"fmt"
	"strconv"
	"strings"
)

// MediumInfo holds parsed output of VBoxManage showmediuminfo.
type MediumInfo struct {
	UUID    string
	Path    string
	Format  string
	SizeMB  int
	Variant string
	State   string
	InUse   bool // whether attached to a VM
}

// CreateMedium creates a virtual disk.
// VBoxManage createmedium disk --filename <path> --size <sizeMB> --format <VDI|VMDK|VHD> --variant <Standard|Fixed>
func CreateMedium(driver Driver, filename string, sizeMB int, format string, variant string) error {
	args := []string{
		"createmedium", "disk",
		"--filename", filename,
		"--size", strconv.Itoa(sizeMB),
		"--format", format,
		"--variant", variant,
	}

	_, _, err := driver.Execute(args...)
	if err != nil {
		return fmt.Errorf("createmedium disk %s: %w", filename, err)
	}
	return nil
}

// ResizeMedium resizes a virtual disk.
// VBoxManage modifymedium disk <path> --resize <sizeMB>
func ResizeMedium(driver Driver, path string, sizeMB int) error {
	_, _, err := driver.Execute("modifymedium", "disk", path, "--resize", strconv.Itoa(sizeMB))
	if err != nil {
		return fmt.Errorf("modifymedium resize %s: %w", path, err)
	}
	return nil
}

// CloseMedium closes/unregisters a medium. If delete is true, also deletes the file.
// VBoxManage closemedium disk <path> [--delete]
func CloseMedium(driver Driver, path string, delete bool) error {
	args := []string{"closemedium", "disk", path}
	if delete {
		args = append(args, "--delete")
	}

	_, _, err := driver.Execute(args...)
	if err != nil {
		return fmt.Errorf("closemedium disk %s: %w", path, err)
	}
	return nil
}

// ShowMediumInfo gets info about a medium by parsing VBoxManage showmediuminfo output.
// VBoxManage showmediuminfo disk <path>
func ShowMediumInfo(driver Driver, path string) (*MediumInfo, error) {
	stdout, _, err := driver.Execute("showmediuminfo", "disk", path)
	if err != nil {
		return nil, fmt.Errorf("showmediuminfo disk %s: %w", path, err)
	}

	return parseMediumInfo(stdout)
}

// parseMediumInfo parses the key-value output from VBoxManage showmediuminfo.
func parseMediumInfo(output string) (*MediumInfo, error) {
	info := &MediumInfo{}
	props := make(map[string]string)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		props[key] = value
	}

	info.UUID = props["UUID"]
	info.Path = props["Location"]
	info.Format = props["Storage format"]

	// Parse logical size: "10240 MBytes"
	if sizeStr, ok := props["Logical size"]; ok {
		sizeStr = strings.TrimSuffix(sizeStr, " MBytes")
		sizeStr = strings.TrimSpace(sizeStr)
		if size, err := strconv.Atoi(sizeStr); err == nil {
			info.SizeMB = size
		}
	}

	// Parse format variant: "dynamic default"
	if variant, ok := props["Format variant"]; ok {
		variant = strings.TrimSpace(variant)
		lower := strings.ToLower(variant)
		if strings.Contains(lower, "fixed") {
			info.Variant = "Fixed"
		} else {
			info.Variant = "Standard"
		}
	}

	// Parse state from Accessible field
	if accessible, ok := props["Accessible"]; ok {
		info.State = strings.TrimSpace(accessible)
	} else {
		info.State = props["State"]
	}

	// Parse In use by VMs: non-empty means in use
	if inUse, ok := props["In use by VMs"]; ok {
		info.InUse = strings.TrimSpace(inUse) != ""
	}

	return info, nil
}
