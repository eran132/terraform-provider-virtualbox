// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provider

import (
	"context"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceVM() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceVMRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "VM name to look up. One of name or uuid must be provided.",
			},
			"uuid": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "VM UUID to look up. One of name or uuid must be provided.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "VM state: running, poweroff, paused, saved, aborted.",
			},
			"cpus": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of virtual CPUs.",
			},
			"memory": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Memory size in MiB.",
			},
			"os_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Guest OS type.",
			},
			"vram": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Video memory in MiB.",
			},
		},
	}
}

func dataSourceVMRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	name := d.Get("name").(string)
	uuid := d.Get("uuid").(string)

	if name == "" && uuid == "" {
		return diag.Errorf("one of `name` or `uuid` must be provided")
	}

	identifier := name
	if identifier == "" {
		identifier = uuid
	}

	tflog.Debug(ctx, "reading VirtualBox VM info", map[string]any{
		"identifier": identifier,
	})

	out, _, err := vboxRun(ctx, "showvminfo", identifier, "--machinereadable")
	if err != nil {
		return diag.Errorf("failed to get VM info for %q: %v", identifier, err)
	}

	props := parseMachineReadable(out)

	if v, ok := props["name"]; ok {
		if err := d.Set("name", v); err != nil {
			return diag.Errorf("unable to set name: %v", err)
		}
	}
	if v, ok := props["UUID"]; ok {
		if err := d.Set("uuid", v); err != nil {
			return diag.Errorf("unable to set uuid: %v", err)
		}
		d.SetId(v)
	}
	if v, ok := props["VMState"]; ok {
		if err := d.Set("status", v); err != nil {
			return diag.Errorf("unable to set status: %v", err)
		}
	}
	if v, ok := props["cpus"]; ok {
		n, _ := strconv.Atoi(v)
		if err := d.Set("cpus", n); err != nil {
			return diag.Errorf("unable to set cpus: %v", err)
		}
	}
	if v, ok := props["memory"]; ok {
		n, _ := strconv.Atoi(v)
		if err := d.Set("memory", n); err != nil {
			return diag.Errorf("unable to set memory: %v", err)
		}
	}
	if v, ok := props["ostype"]; ok {
		if err := d.Set("os_type", v); err != nil {
			return diag.Errorf("unable to set os_type: %v", err)
		}
	}
	// VRAM can appear as "VRAM" or "vram"
	if v, ok := props["VRAM"]; ok {
		n, _ := strconv.Atoi(v)
		if err := d.Set("vram", n); err != nil {
			return diag.Errorf("unable to set vram: %v", err)
		}
	} else if v, ok := props["vram"]; ok {
		n, _ := strconv.Atoi(v)
		if err := d.Set("vram", n); err != nil {
			return diag.Errorf("unable to set vram: %v", err)
		}
	}

	if d.Id() == "" {
		return diag.Errorf("UUID not found in VM info output")
	}

	return nil
}

// parseMachineReadable parses VBoxManage --machinereadable output into a map.
// Lines are in the form key="value" or key=value.
func parseMachineReadable(output string) map[string]string {
	result := make(map[string]string)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Strip surrounding quotes from value
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}

		result[key] = value
	}

	return result
}
