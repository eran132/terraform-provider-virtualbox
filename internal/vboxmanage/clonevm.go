package vboxmanage

import (
	"fmt"
)

// CloneVM creates a clone of an existing VM using VBoxManage clonevm.
// If linked is true, a linked clone (using differencing disks) is created.
// If snapshotName is non-empty, the clone is based on that snapshot.
func CloneVM(driver Driver, sourceNameOrUUID string, targetName string, basefolder string, linked bool, snapshotName string) error {
	args := []string{"clonevm", sourceNameOrUUID, "--name", targetName, "--register"}

	if basefolder != "" {
		args = append(args, "--basefolder", basefolder)
	}
	if linked {
		args = append(args, "--options", "link")
	}
	if snapshotName != "" {
		args = append(args, "--snapshot", snapshotName)
	}

	_, _, err := driver.Execute(args...)
	if err != nil {
		return fmt.Errorf("clonevm %s -> %s: %w", sourceNameOrUUID, targetName, err)
	}
	return nil
}

// CloneHD clones a virtual hard disk from source to target path using
// VBoxManage clonehd.
func CloneHD(driver Driver, source string, target string) error {
	_, _, err := driver.Execute("clonehd", source, target)
	if err != nil {
		return fmt.Errorf("clonehd %s -> %s: %w", source, target, err)
	}
	return nil
}

// SetHDUUID assigns a new random UUID to a virtual hard disk using
// VBoxManage internalcommands sethduuid.
func SetHDUUID(driver Driver, path string) error {
	_, _, err := driver.Execute("internalcommands", "sethduuid", path)
	if err != nil {
		return fmt.Errorf("sethduuid %s: %w", path, err)
	}
	return nil
}
