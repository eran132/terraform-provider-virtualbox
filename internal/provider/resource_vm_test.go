package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccVirtualboxVM_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualboxVMConfig_basic(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckVirtualboxVMExists("virtualbox_vm.test"),
					resource.TestCheckResourceAttr("virtualbox_vm.test", "name", "tf-acc-test-basic"),
					resource.TestCheckResourceAttr("virtualbox_vm.test", "cpus", "1"),
					resource.TestCheckResourceAttr("virtualbox_vm.test", "memory", "256mib"),
					resource.TestCheckResourceAttr("virtualbox_vm.test", "status", "running"),
				),
			},
		},
	})
}

func TestAccVirtualboxVM_update(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualboxVMConfig_basic(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckVirtualboxVMExists("virtualbox_vm.test"),
					resource.TestCheckResourceAttr("virtualbox_vm.test", "cpus", "1"),
					resource.TestCheckResourceAttr("virtualbox_vm.test", "memory", "256mib"),
				),
			},
			{
				Config: testAccVirtualboxVMConfig_updated(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckVirtualboxVMExists("virtualbox_vm.test"),
					resource.TestCheckResourceAttr("virtualbox_vm.test", "cpus", "2"),
					resource.TestCheckResourceAttr("virtualbox_vm.test", "memory", "512mib"),
				),
			},
		},
	})
}

func testAccCheckVirtualboxVMExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("no VM ID is set")
		}
		return nil
	}
}

func testAccVirtualboxVMConfig_basic() string {
	return `
resource "virtualbox_vm" "test" {
  name   = "tf-acc-test-basic"
  image  = "https://app.vagrantup.com/ubuntu/boxes/bionic64/versions/20180903.0.0/providers/virtualbox.box"
  cpus   = 1
  memory = "256mib"

  network_adapter {
    type = "nat"
  }
}
`
}

func testAccVirtualboxVMConfig_updated() string {
	return `
resource "virtualbox_vm" "test" {
  name   = "tf-acc-test-basic"
  image  = "https://app.vagrantup.com/ubuntu/boxes/bionic64/versions/20180903.0.0/providers/virtualbox.box"
  cpus   = 2
  memory = "512mib"

  network_adapter {
    type = "nat"
  }
}
`
}
