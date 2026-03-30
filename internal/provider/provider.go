// Package provider serves as an entrypoint, returning the list of available
// resources for the plugin.
package provider

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/terra-farm/go-virtualbox"
)

func init() {
	// Terraform is already adding the timestamp for us
	log.SetFlags(log.Lshortfile)
	log.SetPrefix(fmt.Sprintf("pid-%d-", os.Getpid()))
}

// New returns a resource provider for virtualbox.
func New() *schema.Provider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"virtualbox_vm":               resourceVM(),
			"virtualbox_disk":             resourceDisk(),
			"virtualbox_snapshot":         resourceSnapshot(),
			"virtualbox_hostonly_network": resourceHostonlyNetwork(),
			"virtualbox_nat_network":      resourceNATNetwork(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"virtualbox_host_info": dataSourceHostInfo(),
			"virtualbox_vm":       dataSourceVM(),
			"virtualbox_network":  dataSourceNetwork(),
		},
		ConfigureContextFunc: configure,
	}
}

// configure creates a new instance of the new virtualbox manager which will be
// used for communication with virtualbox.
func configure(context.Context, *schema.ResourceData) (any, diag.Diagnostics) {
	return virtualbox.NewManager(), nil
}
