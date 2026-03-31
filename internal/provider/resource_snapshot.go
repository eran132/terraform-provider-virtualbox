package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceSnapshot() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSnapshotCreate,
		ReadContext:   resourceSnapshotRead,
		UpdateContext: resourceSnapshotUpdate,
		DeleteContext: resourceSnapshotDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"vm_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "UUID of the VM",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Snapshot name",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "Snapshot description",
			},
			"live": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Take snapshot while VM is running",
			},
			"uuid": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Snapshot UUID assigned by VirtualBox",
			},
		},
	}
}

func resourceSnapshotCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	vmID := d.Get("vm_id").(string)
	name := d.Get("name").(string)
	description := d.Get("description").(string)
	live := d.Get("live").(bool)

	args := []string{"snapshot", vmID, "take", name}
	if description != "" {
		args = append(args, "--description", description)
	}
	if live {
		args = append(args, "--live")
	}

	_, _, err := vboxRun(ctx, args...)
	if err != nil {
		return diag.Errorf("failed to take snapshot %q for VM %s: %v", name, vmID, err)
	}

	// Find the UUID of the snapshot we just created
	snapshots, err := listSnapshots(ctx, vmID)
	if err != nil {
		return diag.Errorf("failed to list snapshots after creation: %v", err)
	}

	var snapshotUUID string
	for _, s := range snapshots {
		if s.name == name {
			snapshotUUID = s.uuid
			break
		}
	}

	if snapshotUUID == "" {
		return diag.Errorf("snapshot %q was created but could not be found in snapshot list", name)
	}

	d.SetId(snapshotUUID)

	if err := d.Set("uuid", snapshotUUID); err != nil {
		return diag.FromErr(err)
	}

	return resourceSnapshotRead(ctx, d, meta)
}

func resourceSnapshotRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	vmID := d.Get("vm_id").(string)
	snapshotUUID := d.Id()

	snapshots, err := listSnapshots(ctx, vmID)
	if err != nil {
		// VM may have been deleted; treat as snapshot gone
		d.SetId("")
		return nil
	}

	var found *snapshotInfo
	for i := range snapshots {
		if snapshots[i].uuid == snapshotUUID {
			found = &snapshots[i]
			break
		}
	}

	if found == nil {
		// Snapshot was deleted outside of Terraform
		d.SetId("")
		return nil
	}

	var diags diag.Diagnostics

	if err := d.Set("name", found.name); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("description", found.description); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}
	if err := d.Set("uuid", found.uuid); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	return diags
}

func resourceSnapshotUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	vmID := d.Get("vm_id").(string)
	name := d.Get("name").(string)

	if d.HasChange("description") {
		newDescription := d.Get("description").(string)
		_, _, err := vboxRun(ctx, "snapshot", vmID, "edit", name, "--description", newDescription)
		if err != nil {
			return diag.Errorf("failed to update description for snapshot %q on VM %s: %v", name, vmID, err)
		}
	}

	return resourceSnapshotRead(ctx, d, meta)
}

func resourceSnapshotDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	vmID := d.Get("vm_id").(string)
	snapshotUUID := d.Id()

	_, _, err := vboxRun(ctx, "snapshot", vmID, "delete", snapshotUUID)
	if err != nil {
		return diag.Errorf("failed to delete snapshot %s on VM %s: %v", snapshotUUID, vmID, err)
	}

	d.SetId("")

	return nil
}

// snapshotInfo holds parsed snapshot data from VBoxManage snapshot list --machinereadable.
type snapshotInfo struct {
	name        string
	uuid        string
	description string
}

// listSnapshots parses the machinereadable snapshot list output.
//
// The output contains lines like:
//
//	SnapshotName="snap1"
//	SnapshotUUID="abc-123"
//	SnapshotDescription="some description"
//	SnapshotName-1="snap2"
//	SnapshotUUID-1="def-456"
//	SnapshotDescription-1=""
//
// The first snapshot has no suffix; subsequent snapshots use -1, -2, etc.
func listSnapshots(ctx context.Context, vmID string) ([]snapshotInfo, error) {
	stdout, _, err := vboxRun(ctx, "snapshot", vmID, "list", "--machinereadable")
	if err != nil {
		// VBoxManage returns an error when there are no snapshots
		if strings.Contains(stdout, "does not have any snapshots") {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list snapshots for VM %s: %w", vmID, err)
	}

	// Also check stdout for "does not have any snapshots" without error
	if strings.Contains(stdout, "does not have any snapshots") {
		return nil, nil
	}

	// Parse key=value pairs into a map keyed by suffix
	props := make(map[string]string)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}
		key := line[:eqIdx]
		value := line[eqIdx+1:]
		// Strip surrounding quotes from value
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		props[key] = value
	}

	// Collect snapshots by matching SnapshotName/SnapshotUUID pairs.
	// The first snapshot has keys like SnapshotName, SnapshotUUID.
	// Subsequent ones use SnapshotName-1, SnapshotName-2, etc.
	var snapshots []snapshotInfo

	// Check for the first (unsuffixed) snapshot
	if name, ok := props["SnapshotName"]; ok {
		s := snapshotInfo{
			name:        name,
			uuid:        props["SnapshotUUID"],
			description: props["SnapshotDescription"],
		}
		snapshots = append(snapshots, s)
	}

	// Check for numbered snapshots (-1, -2, ...)
	for i := 1; ; i++ {
		suffix := fmt.Sprintf("-%d", i)
		name, ok := props["SnapshotName"+suffix]
		if !ok {
			break
		}
		s := snapshotInfo{
			name:        name,
			uuid:        props["SnapshotUUID"+suffix],
			description: props["SnapshotDescription"+suffix],
		}
		snapshots = append(snapshots, s)
	}

	return snapshots, nil
}
