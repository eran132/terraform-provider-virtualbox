---
page_title: "VirtualBox Provider"
subcategory: ""
description: |-
  The VirtualBox provider manages Oracle VirtualBox resources such as virtual machines, disks, snapshots, and networks.
---

# VirtualBox Provider

The VirtualBox provider allows Terraform to manage [Oracle VirtualBox](https://www.virtualbox.org/) resources. It communicates with VirtualBox through the `VBoxManage` CLI, providing full lifecycle management of VMs, disks, snapshots, and networking.

## Requirements

- [Oracle VirtualBox](https://www.virtualbox.org/wiki/Downloads) must be installed on the host machine.
- The `VBoxManage` command must be available on the system `PATH`.

## Example Usage

```hcl
terraform {
  required_providers {
    virtualbox = {
      source  = "eran132/vbox"
      version = "~> 0.3.0"
    }
  }
}

provider "virtualbox" {}

resource "virtualbox_vm" "example" {
  name   = "example-vm"
  image  = "https://app.vagrantup.com/ubuntu/boxes/bionic64/versions/20180903.0.0/providers/virtualbox.box"
  cpus   = 2
  memory = "1024mib"

  network_adapter {
    type = "nat"
  }
}
```

## Provider Configuration

The provider currently requires no configuration arguments. VirtualBox must be installed and `VBoxManage` must be accessible on the system `PATH`.
