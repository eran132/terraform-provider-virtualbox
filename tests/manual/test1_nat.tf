terraform {
  required_providers {
    virtualbox = {
      source = "eran132/virtualbox"
    }
  }
}

# Test 9: RHEL-compatible (Rocky Linux 9) with dnf provisioning
resource "virtualbox_vm" "rhel_test" {
  name   = "tf-test-rhel"
  image  = "https://app.vagrantup.com/generic/boxes/rocky9/versions/4.3.12/providers/virtualbox/amd64/vagrant.box"
  cpus   = 2
  memory = "2048mib"
  gui    = true

  network_adapter {
    type                  = "nat"
    nat_dns_host_resolver = true

    port_forwarding {
      name       = "ssh"
      protocol   = "tcp"
      host_ip    = "127.0.0.1"
      host_port  = 2222
      guest_port = 22
    }
  }

  connection {
    type        = "ssh"
    user        = "vagrant"
    private_key = file("C:/Users/eranmar/.ssh/vagrant_insecure_key")
    host        = "127.0.0.1"
    port        = 2222
    timeout     = "5m"
  }

  provisioner "remote-exec" {
    inline = [
      "echo '=== Rocky Linux 9 (RHEL-compatible) provisioning ==='",
      "cat /etc/os-release | head -3",

      # Verify dnf works
      "sudo dnf check-update -q || true",
      "echo '=== dnf is working ==='",

      # Install Python 3.11 (RHEL 9 has 3.9 default, 3.11 available)
      "sudo dnf install -y -q python3.11 python3.11-pip 2>/dev/null || sudo dnf install -y -q python3 python3-pip 2>/dev/null",
      "python3 --version",

      # Install Node.js 18 from AppStream
      "sudo dnf module enable -y -q nodejs:18 2>/dev/null || true",
      "sudo dnf install -y -q nodejs npm 2>/dev/null",
      "node --version || echo 'node not available'",

      # Install Java 17
      "sudo dnf install -y -q java-17-openjdk java-17-openjdk-devel 2>/dev/null",
      "java -version 2>&1 | head -1",

      # Verify dnf is NOT broken after installations
      "echo '=== Verifying dnf integrity ==='",
      "sudo dnf check 2>&1 | tail -3",
      "sudo rpm -Va --nofiles --nodigest 2>&1 | wc -l",

      # Final verification
      "python3 -c 'import sys; print(f\"Python {sys.version}\")'",
      "echo '=== All RHEL provisioning complete ==='",
    ]
  }
}

output "vm_status" {
  value = virtualbox_vm.rhel_test.status
}
