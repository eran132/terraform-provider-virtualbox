package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	vbox "github.com/terra-farm/go-virtualbox"
)

func resourceDisk() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDiskCreate,
		ReadContext:   resourceDiskRead,
		UpdateContext: resourceDiskUpdate,
		DeleteContext: resourceDiskDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceDiskImport,
		},

		Schema: map[string]*schema.Schema{
			"file_path": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Absolute path for the disk file",
			},
			"size": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Disk size in MiB",
			},
			"format": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "VDI",
				ForceNew:    true,
				Description: "Disk format: VDI, VMDK, or VHD",
				ValidateFunc: func(val any, key string) (warns []string, errs []error) {
					v := val.(string)
					valid := map[string]bool{"VDI": true, "VMDK": true, "VHD": true}
					if !valid[v] {
						errs = append(errs, fmt.Errorf("%q must be one of VDI, VMDK, VHD, got: %s", key, v))
					}
					return
				},
			},
			"variant": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "Standard",
				ForceNew:    true,
				Description: "Disk variant: Standard (dynamic) or Fixed",
				ValidateFunc: func(val any, key string) (warns []string, errs []error) {
					v := val.(string)
					valid := map[string]bool{"Standard": true, "Fixed": true}
					if !valid[v] {
						errs = append(errs, fmt.Errorf("%q must be one of Standard, Fixed, got: %s", key, v))
					}
					return
				},
			},
			"uuid": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "VirtualBox UUID of the disk",
			},
		},
	}
}

func resourceDiskCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	filePath := d.Get("file_path").(string)
	size := d.Get("size").(int)
	format := d.Get("format").(string)
	variant := d.Get("variant").(string)

	// Create the medium
	_, _, err := vbox.Run(ctx, "createmedium", "disk",
		"--filename", filePath,
		"--size", strconv.Itoa(size),
		"--format", format,
		"--variant", variant,
	)
	if err != nil {
		return diag.Errorf("failed to create disk %s: %v", filePath, err)
	}

	// Get the UUID from showmediuminfo
	info, err := showMediumInfo(ctx, filePath)
	if err != nil {
		return diag.Errorf("failed to get disk info after creation: %v", err)
	}

	d.SetId(info.uuid)

	if err := d.Set("uuid", info.uuid); err != nil {
		return diag.FromErr(err)
	}

	return resourceDiskRead(ctx, d, meta)
}

func resourceDiskRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	filePath := d.Get("file_path").(string)

	info, err := showMediumInfo(ctx, filePath)
	if err != nil {
		// Disk no longer exists or is inaccessible
		d.SetId("")
		return nil
	}

	// Disk is inaccessible if explicitly marked so (not present means it's fine)
	if info.accessible == "no" {
		d.SetId("")
		return nil
	}

	var diags diag.Diagnostics

	if err := d.Set("uuid", info.uuid); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("size", info.sizeMB); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("format", info.format); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("variant", info.variant); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	return diags
}

func resourceDiskUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	filePath := d.Get("file_path").(string)

	if d.HasChange("size") {
		newSize := d.Get("size").(int)

		_, _, err := vbox.Run(ctx, "modifymedium", "disk", filePath, "--resize", strconv.Itoa(newSize))
		if err != nil {
			return diag.Errorf("failed to resize disk %s: %v", filePath, err)
		}
	}

	return resourceDiskRead(ctx, d, meta)
}

func resourceDiskDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	filePath := d.Get("file_path").(string)

	_, _, err := vbox.Run(ctx, "closemedium", "disk", filePath, "--delete")
	if err != nil {
		// If the file is already gone (e.g. VM delete cleaned it up), that's fine
		errStr := fmt.Sprintf("%v", err)
		if !strings.Contains(errStr, "VERR_FILE_NOT_FOUND") && !strings.Contains(errStr, "not found") {
			return diag.Errorf("failed to delete disk %s: %v", filePath, err)
		}
	}

	d.SetId("")

	return nil
}

// resourceDiskImport imports a disk by file path.
// Usage: terraform import virtualbox_disk.example /path/to/disk.vdi
func resourceDiskImport(ctx context.Context, d *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
	filePath := d.Id()

	// Set file_path from the import ID
	if err := d.Set("file_path", filePath); err != nil {
		return nil, err
	}

	// Read disk info to get the UUID
	info, err := showMediumInfo(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot import disk %s: %w", filePath, err)
	}

	d.SetId(info.uuid)

	return []*schema.ResourceData{d}, nil
}

// diskMediumInfo holds parsed output from VBoxManage showmediuminfo for a disk.
type diskMediumInfo struct {
	uuid       string
	format     string
	sizeMB     int
	variant    string
	accessible string
}

// showMediumInfo runs VBoxManage showmediuminfo and parses the key-value output.
func showMediumInfo(ctx context.Context, filePath string) (*diskMediumInfo, error) {
	stdout, _, err := vbox.Run(ctx, "showmediuminfo", "disk", filePath)
	if err != nil {
		return nil, fmt.Errorf("showmediuminfo disk %s: %w", filePath, err)
	}

	info := &diskMediumInfo{}
	props := make(map[string]string)

	for _, line := range strings.Split(stdout, "\n") {
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

	info.uuid = props["UUID"]
	info.format = props["Storage format"]
	info.accessible = props["Accessible"]

	// Parse size — may be "Capacity" or "Logical size" depending on VBox version
	for _, sizeKey := range []string{"Capacity", "Logical size"} {
		if sizeStr, ok := props[sizeKey]; ok {
			sizeStr = strings.TrimSuffix(sizeStr, " MBytes")
			sizeStr = strings.TrimSpace(sizeStr)
			if size, err := strconv.Atoi(sizeStr); err == nil {
				info.sizeMB = size
				break
			}
		}
	}

	// Parse format variant: "dynamic default" -> "Standard", "fixed default" -> "Fixed"
	if variant, ok := props["Format variant"]; ok {
		lower := strings.ToLower(strings.TrimSpace(variant))
		if strings.Contains(lower, "fixed") {
			info.variant = "Fixed"
		} else {
			info.variant = "Standard"
		}
	}

	return info, nil
}
