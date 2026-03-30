package vboxmanage

import (
	"fmt"
	"strings"
)

// SnapshotInfo represents a VM snapshot.
type SnapshotInfo struct {
	Name        string
	UUID        string
	Description string
	TimeStamp   string
}

// SnapshotTake creates a new snapshot of a VM.
// If live is true, the snapshot is taken without pausing the VM.
func SnapshotTake(driver Driver, vmNameOrUUID string, name string, description string, live bool) error {
	args := []string{"snapshot", vmNameOrUUID, "take", name}

	if description != "" {
		args = append(args, "--description", description)
	}
	if live {
		args = append(args, "--live")
	}

	_, _, err := driver.Execute(args...)
	if err != nil {
		return fmt.Errorf("snapshot take %s on %s: %w", name, vmNameOrUUID, err)
	}
	return nil
}

// SnapshotRestore restores a VM to a named snapshot.
func SnapshotRestore(driver Driver, vmNameOrUUID string, name string) error {
	_, _, err := driver.Execute("snapshot", vmNameOrUUID, "restore", name)
	if err != nil {
		return fmt.Errorf("snapshot restore %s on %s: %w", name, vmNameOrUUID, err)
	}
	return nil
}

// SnapshotDelete deletes a named snapshot from a VM.
func SnapshotDelete(driver Driver, vmNameOrUUID string, name string) error {
	_, _, err := driver.Execute("snapshot", vmNameOrUUID, "delete", name)
	if err != nil {
		return fmt.Errorf("snapshot delete %s on %s: %w", name, vmNameOrUUID, err)
	}
	return nil
}

// SnapshotList lists all snapshots of a VM.
// Note: This is a basic implementation that parses the machine-readable output.
// A full hierarchical snapshot tree parser will be added in Phase 5.
func SnapshotList(driver Driver, vmNameOrUUID string) ([]SnapshotInfo, error) {
	stdout, _, err := driver.Execute("snapshot", vmNameOrUUID, "list", "--machinereadable")
	if err != nil {
		// VBoxManage returns error if there are no snapshots
		if exitErr, ok := err.(*ExitError); ok {
			if strings.Contains(exitErr.Stderr, "does not have any snapshots") ||
				strings.Contains(exitErr.Stdout, "does not have any snapshots") {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("snapshot list %s: %w", vmNameOrUUID, err)
	}

	return parseSnapshotList(stdout), nil
}

// parseSnapshotList parses the machine-readable snapshot list output.
func parseSnapshotList(output string) []SnapshotInfo {
	var snapshots []SnapshotInfo

	kvs := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.Trim(line[:idx], "\"")
		value := strings.Trim(line[idx+1:], "\"")
		kvs[key] = value
	}

	// Snapshot keys follow the pattern: SnapshotName, SnapshotName-1, SnapshotName-2, etc.
	// and SnapshotUUID, SnapshotDescription, SnapshotTimeStamp
	for i := 0; ; i++ {
		var nameKey, uuidKey, descKey, timeKey string
		if i == 0 {
			nameKey = "SnapshotName"
			uuidKey = "SnapshotUUID"
			descKey = "SnapshotDescription"
			timeKey = "SnapshotTimeStamp"
		} else {
			nameKey = fmt.Sprintf("SnapshotName-%d", i)
			uuidKey = fmt.Sprintf("SnapshotUUID-%d", i)
			descKey = fmt.Sprintf("SnapshotDescription-%d", i)
			timeKey = fmt.Sprintf("SnapshotTimeStamp-%d", i)
		}

		name, exists := kvs[nameKey]
		if !exists {
			break
		}

		snap := SnapshotInfo{
			Name:        name,
			UUID:        kvs[uuidKey],
			Description: kvs[descKey],
			TimeStamp:   kvs[timeKey],
		}
		snapshots = append(snapshots, snap)
	}

	return snapshots
}
