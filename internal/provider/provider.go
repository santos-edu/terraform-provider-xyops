package provider

import (
	"context"
	"os"

	"github.com/santos-edu/terraform-provider-xyops/internal/xyops"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type xyopsProvider struct{ version string }

type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &xyopsProvider{version: version} }
}

func (p *xyopsProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "xyops"
	resp.Version = p.version
}

func (p *xyopsProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for xyOps, a server scheduler and monitor. Set endpoint and api_key in the configuration, or provide XYOPS_ENDPOINT and XYOPS_API_KEY. xyops_server adopts a server that already registered itself and does not create or delete it.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{Optional: true, Description: "xyOps API base URL. Required: set this value or provide XYOPS_ENDPOINT."},
			"api_key":  schema.StringAttribute{Optional: true, Sensitive: true, Description: "xyOps API key. Set this value or provide XYOPS_API_KEY."},
		},
	}
}

func (p *xyopsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// No default endpoint. A fallback such as localhost would send requests
	// to whatever happens to listen there.
	endpoint := providerConfigValue(config.Endpoint, os.Getenv("XYOPS_ENDPOINT"), "")
	if endpoint == "" {
		resp.Diagnostics.AddError("Missing xyOps endpoint", "Set endpoint in the provider configuration or set XYOPS_ENDPOINT.")
		return
	}
	apiKey := providerConfigValue(config.APIKey, os.Getenv("XYOPS_API_KEY"), "")
	if apiKey == "" {
		resp.Diagnostics.AddError("Missing xyOps API key", "Set api_key in the provider configuration or set XYOPS_API_KEY.")
		return
	}

	client, err := xyops.NewClient(endpoint, apiKey)
	if err != nil {
		resp.Diagnostics.AddError("Invalid xyOps endpoint", err.Error())
		return
	}
	if err := client.ValidateKey(); err != nil {
		resp.Diagnostics.AddError("Invalid xyOps API key", err.Error())
		return
	}
	resp.ResourceData = client
	resp.DataSourceData = client
}

func providerConfigValue(configValue types.String, environmentValue, defaultValue string) string {
	if !configValue.IsNull() && !configValue.IsUnknown() {
		return configValue.ValueString()
	}
	if environmentValue != "" {
		return environmentValue
	}
	return defaultValue
}

func (p *xyopsProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAlertResource,
		NewBucketResource,
		NewTagResource,
		NewGroupResource,
		NewCategoryResource,
		NewPluginResource,
		NewWebhookResource,
		NewEventResource,
		NewWorkflowResource,
		NewMonitorResource,
		NewConfigResource,
		NewRoleResource,
		NewAPIKeyResource,
		NewChannelResource,
		NewSecretResource,
		NewServerResource,
	}
}

func (p *xyopsProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewServersDataSource,
		NewJobsDataSource,
		NewAlertsDataSource,
	}
}
