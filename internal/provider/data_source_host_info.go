// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceHostInfo() *schema.Resource {
	interfaceSchema := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"ipv4_address": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"ipv4_netmask": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}

	return &schema.Resource{
		ReadContext: dataSourceHostInfoRead,

		Schema: map[string]*schema.Schema{
			"virtualbox_version": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"host_only_interfaces": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     interfaceSchema,
			},

			"bridged_interfaces": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     interfaceSchema,
			},
		},
	}
}

func dataSourceHostInfoRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	tflog.Debug(ctx, "reading VirtualBox host info")

	// Get VirtualBox version
	versionOut, _, err := vboxRun(ctx, "--version")
	if err != nil {
		return diag.Errorf("failed to get VirtualBox version: %v", err)
	}
	version := strings.TrimSpace(versionOut)
	if err := d.Set("virtualbox_version", version); err != nil {
		return diag.Errorf("unable to set virtualbox_version: %v", err)
	}

	// Get host-only interfaces
	hostOnlyOut, _, err := vboxRun(ctx, "list", "hostonlyifs")
	if err != nil {
		// Not an error if there are simply no host-only interfaces
		tflog.Warn(ctx, "failed to list host-only interfaces", map[string]any{
			"error": err.Error(),
		})
		hostOnlyOut = ""
	}
	hostOnlyIfaces := parseInterfaceBlocks(hostOnlyOut)
	if err := d.Set("host_only_interfaces", hostOnlyIfaces); err != nil {
		return diag.Errorf("unable to set host_only_interfaces: %v", err)
	}

	// Get bridged interfaces
	bridgedOut, _, err := vboxRun(ctx, "list", "bridgedifs")
	if err != nil {
		tflog.Warn(ctx, "failed to list bridged interfaces", map[string]any{
			"error": err.Error(),
		})
		bridgedOut = ""
	}
	bridgedIfaces := parseInterfaceBlocks(bridgedOut)
	if err := d.Set("bridged_interfaces", bridgedIfaces); err != nil {
		return diag.Errorf("unable to set bridged_interfaces: %v", err)
	}

	// Use version as the ID since this is a singleton data source
	d.SetId("host-info-" + version)

	return nil
}

// parseInterfaceBlocks parses VBoxManage output that contains blocks of
// key-value pairs separated by blank lines. Each block represents one
// network interface.
//
// Expected format per block:
//
//	Name:            eth0
//	...
//	IPAddress:       192.168.1.5
//	NetworkMask:     255.255.255.0
//	Status:          Up
func parseInterfaceBlocks(output string) []map[string]any {
	if strings.TrimSpace(output) == "" {
		return []map[string]any{}
	}

	var interfaces []map[string]any
	current := map[string]string{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		// Blank line signals end of a block
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				iface := interfaceBlockToMap(current)
				if iface != nil {
					interfaces = append(interfaces, iface)
				}
				current = map[string]string{}
			}
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		current[key] = value
	}

	// Handle last block if output does not end with a blank line
	if len(current) > 0 {
		iface := interfaceBlockToMap(current)
		if iface != nil {
			interfaces = append(interfaces, iface)
		}
	}

	return interfaces
}

// interfaceBlockToMap converts a parsed key-value block into the schema map
// expected by Terraform.
func interfaceBlockToMap(block map[string]string) map[string]any {
	name, hasName := block["Name"]
	if !hasName {
		return nil
	}

	ipv4 := block["IPAddress"]
	netmask := block["NetworkMask"]
	status := block["Status"]

	return map[string]any{
		"name":         name,
		"ipv4_address": ipv4,
		"ipv4_netmask": netmask,
		"status":       status,
	}
}
