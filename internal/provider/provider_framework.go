package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure FrameworkProvider satisfies various provider interfaces.
var _ provider.Provider = &FrameworkProvider{}

// FrameworkProvider implements the terraform-plugin-framework Provider interface.
// It coexists with the SDK v2 provider via terraform-plugin-mux.
type FrameworkProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// FrameworkProviderModel describes the provider data model.
type FrameworkProviderModel struct {
	VBoxManagePath types.String `tfsdk:"vboxmanage_path"`
}

// NewFrameworkProvider returns a new framework-based provider.
func NewFrameworkProvider(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FrameworkProvider{
			version: version,
		}
	}
}

// Metadata returns the provider type name.
func (p *FrameworkProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "virtualbox"
	resp.Version = p.version
}

// Schema defines the provider-level configuration.
func (p *FrameworkProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for Oracle VirtualBox",
		Attributes: map[string]schema.Attribute{
			"vboxmanage_path": schema.StringAttribute{
				Description: "Path to the VBoxManage binary. If not set, it will be auto-detected from PATH and common install locations.",
				Optional:    true,
			},
		},
	}
}

// Configure prepares the provider for data source and resource operations.
func (p *FrameworkProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data FrameworkProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The VBoxManage path (if set) could be passed to resources/data sources
	// via resp.ResourceData and resp.DataSourceData.
	// For now, resources use auto-detection. This will be used when resources
	// are migrated from SDK to framework.
	if !data.VBoxManagePath.IsNull() {
		resp.ResourceData = data.VBoxManagePath.ValueString()
		resp.DataSourceData = data.VBoxManagePath.ValueString()
	}
}

// Resources defines the resources implemented by this provider.
// Resources are added here as they are migrated from the SDK v2 provider.
func (p *FrameworkProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		// Resources will be added here as they are migrated from SDK v2
	}
}

// DataSources defines the data sources implemented by this provider.
// Data sources are added here as they are migrated from the SDK v2 provider.
func (p *FrameworkProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		// Data sources will be added here as they are migrated from SDK v2
	}
}
