package vboxmanage

import (
	"fmt"
	"regexp"
	"strings"
)

// VMListEntry represents a VM as returned by VBoxManage list vms.
type VMListEntry struct {
	Name string
	UUID string
}

// HostOnlyIF represents a host-only network interface.
type HostOnlyIF struct {
	Name        string
	IPAddress   string
	NetworkMask string
	DHCP        bool
}

// BridgedIF represents a bridged network interface.
type BridgedIF struct {
	Name        string
	IPAddress   string
	NetworkMask string
	Status      string
}

// vmListRegexp matches lines in the format: "VM Name" {uuid}
var vmListRegexp = regexp.MustCompile(`^"(.+)"\s+\{([0-9a-fA-F-]+)\}$`)

// ListVMs returns all registered VMs using VBoxManage list vms.
func ListVMs(driver Driver) ([]VMListEntry, error) {
	stdout, _, err := driver.Execute("list", "vms")
	if err != nil {
		return nil, fmt.Errorf("list vms: %w", err)
	}
	return parseVMList(stdout), nil
}

// ListRunningVMs returns all currently running VMs using VBoxManage list runningvms.
func ListRunningVMs(driver Driver) ([]VMListEntry, error) {
	stdout, _, err := driver.Execute("list", "runningvms")
	if err != nil {
		return nil, fmt.Errorf("list runningvms: %w", err)
	}
	return parseVMList(stdout), nil
}

// ListOSTypes returns a list of supported OS type identifiers.
func ListOSTypes(driver Driver) ([]string, error) {
	stdout, _, err := driver.Execute("list", "ostypes")
	if err != nil {
		return nil, fmt.Errorf("list ostypes: %w", err)
	}

	var types []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID:") {
			id := strings.TrimSpace(strings.TrimPrefix(line, "ID:"))
			if id != "" {
				types = append(types, id)
			}
		}
	}
	return types, nil
}

// ListHostOnlyIFs returns all host-only network interfaces.
func ListHostOnlyIFs(driver Driver) ([]HostOnlyIF, error) {
	stdout, _, err := driver.Execute("list", "hostonlyifs")
	if err != nil {
		return nil, fmt.Errorf("list hostonlyifs: %w", err)
	}
	return parseHostOnlyIFs(stdout), nil
}

// ListBridgedIFs returns all bridged network interfaces.
func ListBridgedIFs(driver Driver) ([]BridgedIF, error) {
	stdout, _, err := driver.Execute("list", "bridgedifs")
	if err != nil {
		return nil, fmt.Errorf("list bridgedifs: %w", err)
	}
	return parseBridgedIFs(stdout), nil
}

// parseVMList parses the output of "VBoxManage list vms" or "list runningvms".
func parseVMList(output string) []VMListEntry {
	var entries []VMListEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := vmListRegexp.FindStringSubmatch(line)
		if matches != nil {
			entries = append(entries, VMListEntry{
				Name: matches[1],
				UUID: matches[2],
			})
		}
	}
	return entries
}

// parseHostOnlyIFs parses the output of "VBoxManage list hostonlyifs".
func parseHostOnlyIFs(output string) []HostOnlyIF {
	var ifs []HostOnlyIF
	var current *HostOnlyIF

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				ifs = append(ifs, *current)
				current = nil
			}
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "Name" {
			current = &HostOnlyIF{Name: value}
		}
		if current == nil {
			continue
		}

		switch key {
		case "IPAddress":
			current.IPAddress = value
		case "NetworkMask":
			current.NetworkMask = value
		case "DHCP":
			current.DHCP = strings.EqualFold(value, "enabled")
		}
	}

	if current != nil {
		ifs = append(ifs, *current)
	}

	return ifs
}

// parseBridgedIFs parses the output of "VBoxManage list bridgedifs".
func parseBridgedIFs(output string) []BridgedIF {
	var ifs []BridgedIF
	var current *BridgedIF

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				ifs = append(ifs, *current)
				current = nil
			}
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "Name" {
			current = &BridgedIF{Name: value}
		}
		if current == nil {
			continue
		}

		switch key {
		case "IPAddress":
			current.IPAddress = value
		case "NetworkMask":
			current.NetworkMask = value
		case "Status":
			current.Status = value
		}
	}

	if current != nil {
		ifs = append(ifs, *current)
	}

	return ifs
}
