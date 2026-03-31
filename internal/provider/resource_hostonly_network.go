package provider

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	vbox "github.com/terra-farm/go-virtualbox"
)

func resourceHostonlyNetwork() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceHostonlyNetworkCreate,
		ReadContext:   resourceHostonlyNetworkRead,
		UpdateContext: resourceHostonlyNetworkUpdate,
		DeleteContext: resourceHostonlyNetworkDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Interface name assigned by VirtualBox (e.g. vboxnet0)",
			},
			"ipv4_address": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "IPv4 address for the interface",
			},
			"ipv4_netmask": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "255.255.255.0",
				Description: "IPv4 netmask",
			},
			"ipv6_address": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "IPv6 address",
			},
			"ipv6_prefix": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     64,
				Description: "IPv6 prefix length",
			},
			"dhcp_enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Enable DHCP server",
			},
			"dhcp_lower_ip": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "DHCP range lower bound",
			},
			"dhcp_upper_ip": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "DHCP range upper bound",
			},
		},
	}
}

func resourceHostonlyNetworkCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	// Create the host-only interface
	stdout, _, err := vbox.Run(ctx, "hostonlyif", "create")
	if err != nil {
		return diag.Errorf("failed to create host-only interface: %v", err)
	}

	// Parse interface name from output: "Interface 'vboxnetN' was successfully created"
	name, err := parseCreatedInterfaceName(stdout)
	if err != nil {
		return diag.FromErr(err)
	}

	// Configure IPv4
	ipv4Address := d.Get("ipv4_address").(string)
	ipv4Netmask := d.Get("ipv4_netmask").(string)

	_, _, err = vbox.Run(ctx, "hostonlyif", "ipconfig", name, "--ip", ipv4Address, "--netmask", ipv4Netmask)
	if err != nil {
		return diag.Errorf("failed to configure IPv4 on %s: %v", name, err)
	}

	// Configure IPv6 if provided
	if v, ok := d.GetOk("ipv6_address"); ok {
		ipv6Address := v.(string)
		ipv6Prefix := strconv.Itoa(d.Get("ipv6_prefix").(int))

		_, _, err = vbox.Run(ctx, "hostonlyif", "ipconfig", name, "--ipv6", ipv6Address, "--ipv6prefix", ipv6Prefix)
		if err != nil {
			return diag.Errorf("failed to configure IPv6 on %s: %v", name, err)
		}
	}

	// Configure DHCP if enabled
	if d.Get("dhcp_enabled").(bool) {
		if diags := addDHCPServer(ctx, d, name); diags.HasError() {
			return diags
		}
	}

	d.SetId(name)

	if err := d.Set("name", name); err != nil {
		return diag.FromErr(err)
	}

	return resourceHostonlyNetworkRead(ctx, d, meta)
}

func resourceHostonlyNetworkRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	name := d.Id()

	stdout, _, err := vbox.Run(ctx, "list", "hostonlyifs")
	if err != nil {
		return diag.Errorf("failed to list host-only interfaces: %v", err)
	}

	iface, found := parseHostonlyInterface(stdout, name)
	if !found {
		// Interface no longer exists
		d.SetId("")
		return nil
	}

	var diags diag.Diagnostics

	if err := d.Set("name", name); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	if err := d.Set("ipv4_address", iface.ipv4Address); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	if err := d.Set("ipv4_netmask", iface.ipv4Netmask); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	if iface.ipv6Address != "" {
		if err := d.Set("ipv6_address", iface.ipv6Address); err != nil {
			diags = append(diags, diag.FromErr(err)...)
		}
	}

	if iface.ipv6Prefix > 0 {
		if err := d.Set("ipv6_prefix", iface.ipv6Prefix); err != nil {
			diags = append(diags, diag.FromErr(err)...)
		}
	}

	if err := d.Set("dhcp_enabled", iface.dhcpEnabled); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	return diags
}

func resourceHostonlyNetworkUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	name := d.Id()

	// Update IPv4 configuration
	if d.HasChange("ipv4_address") || d.HasChange("ipv4_netmask") {
		ipv4Address := d.Get("ipv4_address").(string)
		ipv4Netmask := d.Get("ipv4_netmask").(string)

		_, _, err := vbox.Run(ctx, "hostonlyif", "ipconfig", name, "--ip", ipv4Address, "--netmask", ipv4Netmask)
		if err != nil {
			return diag.Errorf("failed to update IPv4 on %s: %v", name, err)
		}
	}

	// Update IPv6 configuration
	if d.HasChange("ipv6_address") || d.HasChange("ipv6_prefix") {
		if v, ok := d.GetOk("ipv6_address"); ok {
			ipv6Address := v.(string)
			ipv6Prefix := strconv.Itoa(d.Get("ipv6_prefix").(int))

			_, _, err := vbox.Run(ctx, "hostonlyif", "ipconfig", name, "--ipv6", ipv6Address, "--ipv6prefix", ipv6Prefix)
			if err != nil {
				return diag.Errorf("failed to update IPv6 on %s: %v", name, err)
			}
		}
	}

	// Update DHCP configuration
	if d.HasChange("dhcp_enabled") || d.HasChange("dhcp_lower_ip") || d.HasChange("dhcp_upper_ip") {
		dhcpEnabled := d.Get("dhcp_enabled").(bool)
		oldDHCP, _ := d.GetChange("dhcp_enabled")
		wasDHCPEnabled := oldDHCP.(bool)

		if !dhcpEnabled && wasDHCPEnabled {
			// Remove DHCP server
			_, _, err := vbox.Run(ctx, "dhcpserver", "remove", "--ifname", name)
			if err != nil {
				return diag.Errorf("failed to remove DHCP server for %s: %v", name, err)
			}
		} else if dhcpEnabled && !wasDHCPEnabled {
			// Add DHCP server
			if diags := addDHCPServer(ctx, d, name); diags.HasError() {
				return diags
			}
		} else if dhcpEnabled {
			// Modify existing DHCP server
			if diags := modifyDHCPServer(ctx, d, name); diags.HasError() {
				return diags
			}
		}
	}

	return resourceHostonlyNetworkRead(ctx, d, meta)
}

func resourceHostonlyNetworkDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	name := d.Id()

	// Remove DHCP server first if enabled
	if d.Get("dhcp_enabled").(bool) {
		_, _, err := vbox.Run(ctx, "dhcpserver", "remove", "--ifname", name)
		if err != nil {
			return diag.Errorf("failed to remove DHCP server for %s: %v", name, err)
		}
	}

	// Remove the host-only interface
	_, _, err := vbox.Run(ctx, "hostonlyif", "remove", name)
	if err != nil {
		return diag.Errorf("failed to remove host-only interface %s: %v", name, err)
	}

	d.SetId("")

	return nil
}

// parseCreatedInterfaceName extracts the interface name from VBoxManage hostonlyif create output.
// Expected format: "Interface 'vboxnetN' was successfully created"
func parseCreatedInterfaceName(output string) (string, error) {
	re := regexp.MustCompile(`Interface '([^']+)' was successfully created`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return "", fmt.Errorf("could not parse interface name from output: %s", output)
	}

	return matches[1], nil
}

// hostonlyInterfaceInfo holds parsed host-only interface data.
type hostonlyInterfaceInfo struct {
	ipv4Address string
	ipv4Netmask string
	ipv6Address string
	ipv6Prefix  int
	dhcpEnabled bool
}

// parseHostonlyInterface parses VBoxManage list hostonlyifs output and returns info for the named interface.
func parseHostonlyInterface(output, targetName string) (hostonlyInterfaceInfo, bool) {
	var info hostonlyInterfaceInfo

	// Split output into blocks separated by blank lines
	blocks := strings.Split(output, "\n\n")

	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		props := make(map[string]string)

		for _, line := range lines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				props[key] = value
			}
		}

		if props["Name"] != targetName {
			continue
		}

		info.ipv4Address = props["IPAddress"]
		info.ipv4Netmask = props["NetworkMask"]
		info.ipv6Address = props["IPV6Address"]

		if prefixStr, ok := props["IPV6NetworkMaskPrefixLength"]; ok {
			if prefix, err := strconv.Atoi(prefixStr); err == nil {
				info.ipv6Prefix = prefix
			}
		}

		info.dhcpEnabled = strings.EqualFold(props["DHCP"], "Enabled")

		return info, true
	}

	return info, false
}

// addDHCPServer adds a DHCP server for the given host-only interface.
func addDHCPServer(ctx context.Context, d *schema.ResourceData, name string) diag.Diagnostics {
	ipv4Address := d.Get("ipv4_address").(string)
	ipv4Netmask := d.Get("ipv4_netmask").(string)
	lowerIP := d.Get("dhcp_lower_ip").(string)
	upperIP := d.Get("dhcp_upper_ip").(string)

	if lowerIP == "" || upperIP == "" {
		return diag.Errorf("dhcp_lower_ip and dhcp_upper_ip are required when dhcp_enabled is true")
	}

	_, _, err := vbox.Run(ctx, "dhcpserver", "add",
		"--ifname", name,
		"--ip", ipv4Address,
		"--netmask", ipv4Netmask,
		"--lowerip", lowerIP,
		"--upperip", upperIP,
		"--enable",
	)
	if err != nil {
		return diag.Errorf("failed to add DHCP server for %s: %v", name, err)
	}

	return nil
}

// modifyDHCPServer modifies an existing DHCP server for the given host-only interface.
func modifyDHCPServer(ctx context.Context, d *schema.ResourceData, name string) diag.Diagnostics {
	ipv4Address := d.Get("ipv4_address").(string)
	ipv4Netmask := d.Get("ipv4_netmask").(string)
	lowerIP := d.Get("dhcp_lower_ip").(string)
	upperIP := d.Get("dhcp_upper_ip").(string)

	if lowerIP == "" || upperIP == "" {
		return diag.Errorf("dhcp_lower_ip and dhcp_upper_ip are required when dhcp_enabled is true")
	}

	_, _, err := vbox.Run(ctx, "dhcpserver", "modify",
		"--ifname", name,
		"--ip", ipv4Address,
		"--netmask", ipv4Netmask,
		"--lowerip", lowerIP,
		"--upperip", upperIP,
		"--enable",
	)
	if err != nil {
		return diag.Errorf("failed to modify DHCP server for %s: %v", name, err)
	}

	return nil
}
