# CLAUDE.md - terraform-provider-vbox

## Project Overview
Terraform provider for Oracle VirtualBox. Published as `eran132/vbox` on the Terraform Registry. Forked from `terra-farm/terraform-provider-virtualbox`. Production-grade provider with Vagrant-level feature parity.

## Build & Test Commands
```bash
go build ./...     # Build all packages
go vet ./...       # Vet all packages
go test ./...      # Run all tests
go test ./internal/vboxmanage/  # VBoxManage layer tests only
make build         # Build the provider binary
make test          # Unit tests with race detector + coverage
make testacc       # Acceptance tests (requires TF_ACC=1 and VirtualBox)
```

## Architecture
- **Language**: Go 1.25+
- **Frameworks**: Terraform Plugin SDK v2 + Plugin Framework v1.19 (via terraform-plugin-mux)
- **VBoxManage interaction**: Custom command layer (`internal/vboxmanage/`) + `terra-farm/go-virtualbox`
- **Entry point**: `main.go` — mux server at `registry.terraform.io/eran132/vbox`
- **Provider code**: `internal/provider/` (15+ Go files)
- **VBoxManage wrappers**: `internal/vboxmanage/` (13 Go files)

## Provider Resources & Data Sources

### Resources (5):
- `virtualbox_vm` — Full VM lifecycle (30+ configurable attributes)
- `virtualbox_disk` — Standalone virtual disk management
- `virtualbox_snapshot` — VM snapshot management
- `virtualbox_hostonly_network` — Host-only network interfaces
- `virtualbox_nat_network` — NAT networks with port forwarding

### Data Sources (3):
- `virtualbox_host_info` — VirtualBox version, available interfaces
- `virtualbox_vm` — Look up existing VMs by name/UUID
- `virtualbox_network` — List all available networks

## Key VM Attributes
- **Basic**: name, image, cpus, memory, status, boot_order
- **System**: os_type, firmware (bios/efi), chipset, vram, graphics_controller, gui
- **CPU**: cpu_execution_cap, ioapic, pae, nested_hw_virt, largepages, vtx_vpid
- **Networking**: network_adapter with port_forwarding, promiscuous_mode, cable_connected, NAT DNS
- **Storage**: storage_controller, disk_attachment, optical_disks
- **Sharing**: shared_folder (host_path, auto_mount, mount_point)
- **Devices**: usb_controller, serial_port, clipboard_mode, drag_and_drop
- **Provisioning**: user_data (cloud-init via guest properties)
- **Advanced**: customize (arbitrary VBoxManage commands), linked_clone, source_vm
- **Import**: all 5 resources support `terraform import`

## File Structure
```
internal/
  provider/
    provider.go              # SDK provider registration
    provider_framework.go    # Framework provider (mux)
    provider_test.go         # Test infrastructure
    resource_vm.go           # VM schema
    resource_vm_crud.go      # VM CRUD operations
    resource_vm_network.go   # Network conversion + NIC settings
    resource_vm_helpers.go   # tfToVbox, waitForVM, fetchRemote
    resource_vm_customize.go # Customize, firmware, USB, serial, chipset
    resource_vm_shared_folders.go # Shared folders, CPU cap, nested HW
    resource_vm_cloud_init.go # Cloud-init user_data
    resource_vm_test.go      # Acceptance tests
    resource_disk.go         # Disk resource
    resource_snapshot.go     # Snapshot resource
    resource_hostonly_network.go
    resource_nat_network.go
    data_source_host_info.go
    data_source_vm.go
    data_source_network.go
    image.go                 # Image download/unpack
    image_test.go
  vboxmanage/
    runner.go + runner_test.go  # Driver interface, VBoxManage execution
    modifyvm.go, showvminfo.go, createvm.go
    storagectl.go, storageattach.go, medium.go
    guestproperty.go, list.go, clonevm.go
    snapshot.go, startvm.go
```

## CI/CD
- GitHub Actions: lint (golangci-lint v6) + build matrix (Ubuntu/Windows/macOS, Go 1.22/1.23)
- GoReleaser v6 for cross-platform releases with GPG signing (crazy-max/ghaction-import-gpg@v6)
- Registry: `eran132/vbox`

## License
MIT (original copyright 2016 ccll, 2026 eran132)
