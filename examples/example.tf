# See the examples/ subdirectories for usage:
#   examples/basic/          - Simple VM
#   examples/port-forwarding/ - NAT with port forwarding
#   examples/multi-disk/     - Additional disks
#   examples/windows-vm/     - Windows with EFI
#   examples/complete/       - All features

# Basic example:
resource "virtualbox_vm" "node" {
  count  = 2
  name   = format("node-%02d", count.index + 1)
  image  = "https://app.vagrantup.com/ubuntu/boxes/bionic64/versions/20180903.0.0/providers/virtualbox.box"
  cpus   = 2
  memory = "512mib"

  network_adapter {
    type           = "hostonly"
    host_interface = "vboxnet1"
  }
}

output "IPAddr" {
  value = virtualbox_vm.node[*].network_adapter.0.ipv4_address
}
