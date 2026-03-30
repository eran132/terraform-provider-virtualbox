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
	vbox "github.com/terra-farm/go-virtualbox"
)

func dataSourceNetwork() *schema.Resource {
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

	natNetworkSchema := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"network": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Network CIDR.",
			},
			"dhcp": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"enabled": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"ipv6": {
				Type:     schema.TypeBool,
				Computed: true,
			},
		},
	}

	return &schema.Resource{
		ReadContext: dataSourceNetworkRead,

		Schema: map[string]*schema.Schema{
			"host_only_networks": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     interfaceSchema,
			},
			"nat_networks": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     natNetworkSchema,
			},
			"bridged_interfaces": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     interfaceSchema,
			},
		},
	}
}

func dataSourceNetworkRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	tflog.Debug(ctx, "reading VirtualBox network info")

	// Host-only interfaces
	hostOnlyOut, _, err := vbox.Run(ctx, "list", "hostonlyifs")
	if err != nil {
		tflog.Warn(ctx, "failed to list host-only interfaces", map[string]any{
			"error": err.Error(),
		})
		hostOnlyOut = ""
	}
	hostOnlyNets := parseHostOnlyBlocks(hostOnlyOut)
	if err := d.Set("host_only_networks", hostOnlyNets); err != nil {
		return diag.Errorf("unable to set host_only_networks: %v", err)
	}

	// NAT networks
	natOut, _, err := vbox.Run(ctx, "list", "natnets")
	if err != nil {
		tflog.Warn(ctx, "failed to list NAT networks", map[string]any{
			"error": err.Error(),
		})
		natOut = ""
	}
	natNets := parseNATNetworkBlocks(natOut)
	if err := d.Set("nat_networks", natNets); err != nil {
		return diag.Errorf("unable to set nat_networks: %v", err)
	}

	// Bridged interfaces
	bridgedOut, _, err := vbox.Run(ctx, "list", "bridgedifs")
	if err != nil {
		tflog.Warn(ctx, "failed to list bridged interfaces", map[string]any{
			"error": err.Error(),
		})
		bridgedOut = ""
	}
	bridgedIfaces := parseBridgedBlocks(bridgedOut)
	if err := d.Set("bridged_interfaces", bridgedIfaces); err != nil {
		return diag.Errorf("unable to set bridged_interfaces: %v", err)
	}

	d.SetId("virtualbox-networks")

	return nil
}

// parseVBoxBlocks splits VBoxManage list output into blocks separated by blank
// lines. Each block is a map of key-value pairs parsed by splitting on the
// first colon.
func parseVBoxBlocks(output string) []map[string]string {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	var blocks []map[string]string
	current := make(map[string]string)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")

		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				blocks = append(blocks, current)
				current = make(map[string]string)
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
		blocks = append(blocks, current)
	}

	return blocks
}

func parseHostOnlyBlocks(output string) []map[string]any {
	blocks := parseVBoxBlocks(output)
	var result []map[string]any

	for _, b := range blocks {
		name, ok := b["Name"]
		if !ok {
			continue
		}
		result = append(result, map[string]any{
			"name":         name,
			"ipv4_address": b["IPAddress"],
			"ipv4_netmask": b["NetworkMask"],
			"status":       b["Status"],
		})
	}

	if result == nil {
		return []map[string]any{}
	}
	return result
}

func parseNATNetworkBlocks(output string) []map[string]any {
	blocks := parseVBoxBlocks(output)
	var result []map[string]any

	for _, b := range blocks {
		name := b["NetworkName"]
		if name == "" {
			continue
		}
		result = append(result, map[string]any{
			"name":    name,
			"network": b["Network"],
			"dhcp":    parseBoolField(b["DHCP Enabled"]),
			"enabled": parseBoolField(b["Enabled"]),
			"ipv6":    parseBoolField(b["IPv6 Enabled"]),
		})
	}

	if result == nil {
		return []map[string]any{}
	}
	return result
}

func parseBridgedBlocks(output string) []map[string]any {
	blocks := parseVBoxBlocks(output)
	var result []map[string]any

	for _, b := range blocks {
		name, ok := b["Name"]
		if !ok {
			continue
		}
		result = append(result, map[string]any{
			"name":         name,
			"ipv4_address": b["IPAddress"],
			"ipv4_netmask": b["NetworkMask"],
			"status":       b["Status"],
		})
	}

	if result == nil {
		return []map[string]any{}
	}
	return result
}

// parseBoolField interprets common VBoxManage boolean strings.
func parseBoolField(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	return v == "yes" || v == "true" || v == "enabled" || v == "1"
}
