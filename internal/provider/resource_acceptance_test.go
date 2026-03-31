package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// ============================================================
// virtualbox_disk tests
// ============================================================

func TestAccVirtualboxDisk_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccDiskConfig_basic(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("virtualbox_disk.test", "uuid"),
					resource.TestCheckResourceAttr("virtualbox_disk.test", "size", "1024"),
					resource.TestCheckResourceAttr("virtualbox_disk.test", "format", "VDI"),
					resource.TestCheckResourceAttr("virtualbox_disk.test", "variant", "Standard"),
				),
			},
		},
	})
}

func testAccDiskConfig_basic() string {
	return `
resource "virtualbox_disk" "test" {
  file_path = pathexpand("~/.terraform/virtualbox/test-acc-disk.vdi")
  size      = 1024
  format    = "VDI"
}
`
}

// ============================================================
// virtualbox_vm with port forwarding
// ============================================================

func TestAccVirtualboxVM_portForwarding(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccVMConfig_portForwarding(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckVirtualboxVMExists("virtualbox_vm.test_pf"),
					resource.TestCheckResourceAttr("virtualbox_vm.test_pf", "name", "tf-acc-test-pf"),
					resource.TestCheckResourceAttr("virtualbox_vm.test_pf", "status", "running"),
					resource.TestCheckResourceAttr("virtualbox_vm.test_pf", "network_adapter.0.type", "nat"),
					resource.TestCheckResourceAttr("virtualbox_vm.test_pf", "network_adapter.0.nat_dns_host_resolver", "true"),
				),
			},
		},
	})
}

func testAccVMConfig_portForwarding() string {
	return `
resource "virtualbox_vm" "test_pf" {
  name   = "tf-acc-test-pf"
  image  = "https://app.vagrantup.com/ubuntu/boxes/bionic64/versions/20180903.0.0/providers/virtualbox.box"
  cpus   = 1
  memory = "256mib"
  gui    = true

  network_adapter {
    type                  = "nat"
    nat_dns_host_resolver = true

    port_forwarding {
      name       = "ssh"
      protocol   = "tcp"
      host_ip    = "127.0.0.1"
      host_port  = 2250
      guest_port = 22
    }

    port_forwarding {
      name       = "http"
      protocol   = "tcp"
      host_port  = 8090
      guest_port = 80
    }
  }
}
`
}

// ============================================================
// virtualbox_vm with shared folder
// ============================================================

func TestAccVirtualboxVM_sharedFolder(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccVMConfig_sharedFolder(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckVirtualboxVMExists("virtualbox_vm.test_sf"),
					resource.TestCheckResourceAttr("virtualbox_vm.test_sf", "name", "tf-acc-test-sf"),
					resource.TestCheckResourceAttr("virtualbox_vm.test_sf", "status", "running"),
				),
			},
		},
	})
}

func testAccVMConfig_sharedFolder() string {
	return `
resource "virtualbox_vm" "test_sf" {
  name   = "tf-acc-test-sf"
  image  = "https://app.vagrantup.com/ubuntu/boxes/bionic64/versions/20180903.0.0/providers/virtualbox.box"
  cpus   = 1
  memory = "256mib"
  gui    = true

  network_adapter {
    type = "nat"
  }

  shared_folder {
    name       = "testshare"
    host_path  = "C:\\Users"
    auto_mount = true
  }
}
`
}

// ============================================================
// virtualbox_vm with snapshot
// ============================================================

func TestAccVirtualboxVM_withSnapshot(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccVMConfig_withSnapshot(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckVirtualboxVMExists("virtualbox_vm.test_snap"),
					resource.TestCheckResourceAttrSet("virtualbox_snapshot.test_snap", "uuid"),
					resource.TestCheckResourceAttr("virtualbox_snapshot.test_snap", "name", "acc-test-snapshot"),
				),
			},
		},
	})
}

func testAccVMConfig_withSnapshot() string {
	return `
resource "virtualbox_vm" "test_snap" {
  name   = "tf-acc-test-snap"
  image  = "https://app.vagrantup.com/ubuntu/boxes/bionic64/versions/20180903.0.0/providers/virtualbox.box"
  cpus   = 1
  memory = "256mib"
  gui    = true

  network_adapter {
    type = "nat"
  }
}

resource "virtualbox_snapshot" "test_snap" {
  vm_id       = virtualbox_vm.test_snap.id
  name        = "acc-test-snapshot"
  description = "Acceptance test snapshot"
}
`
}

// ============================================================
// virtualbox_vm with customize
// ============================================================

func TestAccVirtualboxVM_customize(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccVMConfig_customize(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckVirtualboxVMExists("virtualbox_vm.test_custom"),
					resource.TestCheckResourceAttr("virtualbox_vm.test_custom", "name", "tf-acc-test-custom"),
					resource.TestCheckResourceAttr("virtualbox_vm.test_custom", "status", "running"),
				),
			},
		},
	})
}

func testAccVMConfig_customize() string {
	return `
resource "virtualbox_vm" "test_custom" {
  name   = "tf-acc-test-custom"
  image  = "https://app.vagrantup.com/ubuntu/boxes/bionic64/versions/20180903.0.0/providers/virtualbox.box"
  cpus   = 1
  memory = "256mib"
  gui    = true

  customize = [
    ["modifyvm", ":id", "--description", "Acceptance test VM"],
  ]

  network_adapter {
    type = "nat"
  }
}
`
}

// ============================================================
// virtualbox_vm import test
// ============================================================

func TestAccVirtualboxVM_importBasic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualboxVMConfig_basic(),
			},
			{
				ResourceName:      "virtualbox_vm.test",
				ImportState:       true,
				ImportStateVerify: true,
				// These attributes can't be read back from VBoxManage showvminfo
				ImportStateVerifyIgnore: []string{
					"image", "url", "user_data", "checksum", "checksum_type",
					"customize", "linked_clone", "source_vm", "ova_source",
					"gui", "network_adapter", "optical_disks", "boot_order",
					"storage_controller", "disk_attachment", "shared_folder",
					"serial_port",
				},
			},
		},
	})
}
