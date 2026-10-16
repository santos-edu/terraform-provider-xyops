package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/santos-edu/terraform-provider-xyops/internal/xyops"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*configResource)(nil)
	_ resource.ResourceWithConfigure   = (*configResource)(nil)
	_ resource.ResourceWithImportState = (*configResource)(nil)
)

type configResource struct{ client *xyops.Client }

type configModel struct {
	ID                  types.String `tfsdk:"id"`
	JobUniversalActions types.String `tfsdk:"job_universal_actions"`
}

type configResponse struct {
	Config configPayload `json:"config"`
}

type configPayload struct {
	JobUniversalActions json.RawMessage `json:"job_universal_actions"`
}

const configSyntheticID = "global"

func NewConfigResource() resource.Resource { return &configResource{} }

func (r *configResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_config"
}

func (r *configResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true},
		"job_universal_actions": schema.StringAttribute{
			Required:   true,
			Validators: []validator.String{jsonStringValidator{}},
		},
	}}
}

func (r *configResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*xyops.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *xyops.Client, got %T.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *configResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan configModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating config.")
		return
	}
	actions, diags := parseConfigJSON(plan.JobUniversalActions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Call("admin_update_config", map[string]any{"job_universal_actions": actions}, nil); err != nil {
		resp.Diagnostics.AddError("Create xyOps config failed", err.Error())
		return
	}
	plan.ID = types.StringValue(configSyntheticID)
	resp.Diagnostics.Append(verifyConfigWrite(ctx, r.client, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *configResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state configModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading config.")
		return
	}
	var result configResponse
	err := r.client.Get("admin_get_config", &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps config failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setConfigModel(&state, result.Config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *configResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state configModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating config.")
		return
	}
	resolveUnknowns(&plan, &state)
	actions, diags, changed := jsonStringPayload(ctx, plan.JobUniversalActions, state.JobUniversalActions, "job_universal_actions")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if changed {
		if err := r.client.Call("admin_update_config", map[string]any{"job_universal_actions": actions}, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps config failed", err.Error())
			return
		}
		resp.Diagnostics.Append(verifyConfigWrite(ctx, r.client, &plan)...)
	} else {
		resolveUnknowns(&plan, &state)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = types.StringValue(configSyntheticID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// verifyConfigWrite reads the config back after a write and adopts the server
// answer as state. admin_update_config replies code 0 without returning the
// record, so without the read the state would carry exactly what the plan
// claimed; a popularly cached or normalized value in xyOps then became a
// phantom drift on the next refresh (the TestAccConfigResource_drift flake).
func verifyConfigWrite(ctx context.Context, client *xyops.Client, model *configModel) diag.Diagnostics {
	var diags diag.Diagnostics
	var result configResponse
	if err := client.Get("admin_get_config", &result); err != nil {
		diags.AddError("Verify xyOps config write failed", err.Error())
		return diags
	}
	diags = setConfigModel(model, result.Config)
	return diags
}

func (r *configResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state configModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting config.")
		return
	}
	empty := map[string]any{"default": []any{}, "workflow": []any{}}
	if err := r.client.Call("admin_update_config", map[string]any{"job_universal_actions": empty}, nil); err != nil {
		resp.Diagnostics.AddError("Delete xyOps config failed", err.Error())
	}
}

func (r *configResource) ImportState(_ context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError("Import not supported", "xyops_config is a singleton and cannot be imported.")
}

func parseConfigJSON(value types.String) (any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var actions any
	if err := json.Unmarshal([]byte(value.ValueString()), &actions); err != nil {
		diags.AddAttributeError(path.Root("job_universal_actions"), "Invalid JSON", err.Error())
		return nil, diags
	}
	return actions, diags
}

func setConfigModel(model *configModel, config configPayload) diag.Diagnostics {
	var diags diag.Diagnostics
	model.ID = types.StringValue(configSyntheticID)
	model.JobUniversalActions = jsonStringFromRaw(config.JobUniversalActions, model.JobUniversalActions)
	return diags
}
