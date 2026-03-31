// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceNATNetwork() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceNATNetworkCreate,
		ReadContext:   resourceNATNetworkRead,
		UpdateContext: resourceNATNetworkUpdate,
		DeleteContext: resourceNATNetworkDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"network": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsCIDR,
			},

			"dhcp_enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},

			"ipv6": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},

			"enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},

			"port_forwarding": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"protocol": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice([]string{"tcp", "udp"}, true),
						},
						"host_ip": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						"host_port": {
							Type:         schema.TypeInt,
							Required:     true,
							ValidateFunc: validation.IsPortNumber,
						},
						"guest_ip": {
							Type:     schema.TypeString,
							Required: true,
						},
						"guest_port": {
							Type:         schema.TypeInt,
							Required:     true,
							ValidateFunc: validation.IsPortNumber,
						},
					},
				},
			},
		},
	}
}

func boolToOnOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func resourceNATNetworkCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	name := d.Get("name").(string)
	network := d.Get("network").(string)
	dhcp := d.Get("dhcp_enabled").(bool)
	ipv6 := d.Get("ipv6").(bool)
	enabled := d.Get("enabled").(bool)

	tflog.Debug(ctx, "creating NAT network", map[string]any{
		"name":    name,
		"network": network,
	})

	enableFlag := "--enable"
	if !enabled {
		enableFlag = "--disable"
	}

	args := []string{
		"natnetwork", "add",
		"--netname", name,
		"--network", network,
		enableFlag,
		"--dhcp", boolToOnOff(dhcp),
	}

	if ipv6 {
		args = append(args, "--ipv6", "on")
	}

	if _, _, err := vboxRun(ctx, args...); err != nil {
		return diag.Errorf("failed to create NAT network %q: %v", name, err)
	}

	// Add port forwarding rules
	if err := addPortForwardingRules(ctx, name, d); err != nil {
		return diag.Errorf("failed to add port forwarding rules: %v", err)
	}

	d.SetId(name)

	return resourceNATNetworkRead(ctx, d, meta)
}

func resourceNATNetworkRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	name := d.Id()

	tflog.Debug(ctx, "reading NAT network", map[string]any{"name": name})

	stdout, _, err := vboxRun(ctx, "natnetwork", "list", name)
	if err != nil {
		// If the network no longer exists, remove from state.
		if strings.Contains(stdout, "does not exist") || strings.Contains(fmt.Sprintf("%v", err), "does not exist") {
			d.SetId("")
			return nil
		}
		return diag.Errorf("failed to read NAT network %q: %v", name, err)
	}

	// Parse the output
	if err := parseNATNetworkList(d, stdout); err != nil {
		return diag.Errorf("failed to parse NAT network info for %q: %v", name, err)
	}

	return nil
}

func resourceNATNetworkUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	name := d.Id()

	tflog.Debug(ctx, "updating NAT network", map[string]any{"name": name})

	args := []string{"natnetwork", "modify", "--netname", name}

	if d.HasChange("network") {
		args = append(args, "--network", d.Get("network").(string))
	}

	if d.HasChange("dhcp_enabled") {
		args = append(args, "--dhcp", boolToOnOff(d.Get("dhcp_enabled").(bool)))
	}

	if d.HasChange("ipv6") {
		args = append(args, "--ipv6", boolToOnOff(d.Get("ipv6").(bool)))
	}

	if d.HasChange("enabled") {
		if d.Get("enabled").(bool) {
			args = append(args, "--enable")
		} else {
			args = append(args, "--disable")
		}
	}

	// Only run modify if we have changes beyond the base args
	if len(args) > 4 {
		if _, _, err := vboxRun(ctx, args...); err != nil {
			return diag.Errorf("failed to modify NAT network %q: %v", name, err)
		}
	}

	// Handle port forwarding changes
	if d.HasChange("port_forwarding") {
		old, _ := d.GetChange("port_forwarding")
		oldRules := old.([]any)

		// Delete old rules
		for _, r := range oldRules {
			rule := r.(map[string]any)
			ruleName := rule["name"].(string)
			if _, _, err := vboxRun(ctx, "natnetwork", "modify", "--netname", name, "--port-forward-4", "delete", ruleName); err != nil {
				tflog.Warn(ctx, "failed to delete port forwarding rule", map[string]any{
					"rule":  ruleName,
					"error": err.Error(),
				})
			}
		}

		// Add new rules
		if err := addPortForwardingRules(ctx, name, d); err != nil {
			return diag.Errorf("failed to add port forwarding rules: %v", err)
		}
	}

	return resourceNATNetworkRead(ctx, d, meta)
}

func resourceNATNetworkDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	name := d.Id()

	tflog.Debug(ctx, "deleting NAT network", map[string]any{"name": name})

	if _, _, err := vboxRun(ctx, "natnetwork", "remove", "--netname", name); err != nil {
		return diag.Errorf("failed to delete NAT network %q: %v", name, err)
	}

	d.SetId("")
	return nil
}

func addPortForwardingRules(ctx context.Context, name string, d *schema.ResourceData) error {
	rules, ok := d.GetOk("port_forwarding")
	if !ok {
		return nil
	}

	for _, r := range rules.([]any) {
		rule := r.(map[string]any)
		ruleName := rule["name"].(string)
		protocol := rule["protocol"].(string)
		hostIP := rule["host_ip"].(string)
		hostPort := rule["host_port"].(int)
		guestIP := rule["guest_ip"].(string)
		guestPort := rule["guest_port"].(int)

		ruleStr := fmt.Sprintf("%s:%s:[%s]:%d:[%s]:%d",
			ruleName, protocol, hostIP, hostPort, guestIP, guestPort)

		if _, _, err := vboxRun(ctx, "natnetwork", "modify", "--netname", name, "--port-forward-4", ruleStr); err != nil {
			return fmt.Errorf("failed to add port forwarding rule %q: %w", ruleName, err)
		}
	}

	return nil
}

// parseNATNetworkList parses the output of "VBoxManage natnetwork list <name>"
// and sets the appropriate attributes on the resource data.
func parseNATNetworkList(d *schema.ResourceData, output string) error {
	lines := strings.Split(output, "\n")

	var portForwardingRules []map[string]any

	for _, line := range lines {
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

		switch key {
		case "NetworkName", "Name":
			if err := d.Set("name", value); err != nil {
				return fmt.Errorf("unable to set name: %w", err)
			}
		case "Network":
			if err := d.Set("network", value); err != nil {
				return fmt.Errorf("unable to set network: %w", err)
			}
		case "DHCP Enabled", "DHCP Server":
			dhcp := strings.EqualFold(value, "yes") || strings.EqualFold(value, "on") || strings.EqualFold(value, "true")
			if err := d.Set("dhcp_enabled", dhcp); err != nil {
				return fmt.Errorf("unable to set dhcp_enabled: %w", err)
			}
		case "IPv6 Enabled", "IPv6":
			ipv6 := strings.EqualFold(value, "yes") || strings.EqualFold(value, "on") || strings.EqualFold(value, "true")
			if err := d.Set("ipv6", ipv6); err != nil {
				return fmt.Errorf("unable to set ipv6: %w", err)
			}
		case "Enabled":
			enabled := strings.EqualFold(value, "yes") || strings.EqualFold(value, "on") || strings.EqualFold(value, "true")
			if err := d.Set("enabled", enabled); err != nil {
				return fmt.Errorf("unable to set enabled: %w", err)
			}
		}

		// Parse port forwarding rules from loopback mappings
		// Format: "rulename:proto:[hostip]:hostport:[guestip]:guestport"
		if strings.HasPrefix(key, "Port-forwarding") || strings.HasPrefix(line, "  ") && strings.Contains(line, ":tcp:") || strings.Contains(line, ":udp:") {
			rule := parsePortForwardingRule(line)
			if rule != nil {
				portForwardingRules = append(portForwardingRules, rule)
			}
		}
	}

	if len(portForwardingRules) > 0 {
		if err := d.Set("port_forwarding", portForwardingRules); err != nil {
			return fmt.Errorf("unable to set port_forwarding: %w", err)
		}
	}

	return nil
}

// parsePortForwardingRule parses a port forwarding rule string.
// Expected format: "rulename:proto:[hostip]:hostport:[guestip]:guestport"
func parsePortForwardingRule(line string) map[string]any {
	// The rule may be a value after a key, or a standalone line
	ruleStr := line
	if idx := strings.Index(line, ":"); idx > 0 {
		// Check if the part before the first colon looks like a key (contains spaces)
		prefix := line[:idx]
		if strings.Contains(prefix, " ") {
			ruleStr = strings.TrimSpace(line[idx+1:])
		}
	}

	// Split the rule: name:proto:[hostip]:hostport:[guestip]:guestport
	// We need to handle brackets around IPs
	ruleStr = strings.TrimSpace(ruleStr)
	if ruleStr == "" {
		return nil
	}

	// Remove brackets from IPs for parsing
	cleaned := strings.ReplaceAll(ruleStr, "[", "")
	cleaned = strings.ReplaceAll(cleaned, "]", "")

	parts := strings.Split(cleaned, ":")
	if len(parts) < 6 {
		return nil
	}

	hostPort := 0
	guestPort := 0
	if _, err := fmt.Sscanf(parts[3], "%d", &hostPort); err != nil {
		return nil
	}
	if _, err := fmt.Sscanf(parts[5], "%d", &guestPort); err != nil {
		return nil
	}

	return map[string]any{
		"name":       parts[0],
		"protocol":   strings.ToLower(parts[1]),
		"host_ip":    parts[2],
		"host_port":  hostPort,
		"guest_ip":   parts[4],
		"guest_port": guestPort,
	}
}
