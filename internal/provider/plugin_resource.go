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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*pluginResource)(nil)
	_ resource.ResourceWithConfigure   = (*pluginResource)(nil)
	_ resource.ResourceWithImportState = (*pluginResource)(nil)
)

type pluginResource struct{ client *xyops.Client }

type pluginModel struct {
	ID         types.String `tfsdk:"id"`
	Title      types.String `tfsdk:"title"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Type       types.String `tfsdk:"type"`
	Command    types.String `tfsdk:"command"`
	ScriptFile types.String `tfsdk:"script_file"`
	UID        types.Int64  `tfsdk:"uid"`
	GID        types.Int64  `tfsdk:"gid"`
	Params     types.String `tfsdk:"params"`
	Groups     types.List   `tfsdk:"groups"`
	Format     types.String `tfsdk:"format"`
}

type pluginResponse struct {
	Plugin pluginPayload `json:"plugin"`
}

type pluginPayload struct {
	ID      string          `json:"id"`
	Title   string          `json:"title"`
	Enabled bool            `json:"enabled"`
	Type    string          `json:"type"`
	Command string          `json:"command"`
	Script  string          `json:"script"`
	UID     *int64          `json:"uid"`
	GID     *int64          `json:"gid"`
	Params  json.RawMessage `json:"params"`
	Groups  []string        `json:"groups"`
	Format  string          `json:"format"`
}

func NewPluginResource() resource.Resource { return &pluginResource{} }

func (r *pluginResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_plugin"
}

func (r *pluginResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":          schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":       schema.StringAttribute{Required: true},
		"enabled":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"type":        schema.StringAttribute{Required: true, Validators: []validator.String{enumValidator{allowed: []string{"event", "monitor"}}}},
		"command":     schema.StringAttribute{Optional: true},
		"script_file": schema.StringAttribute{Required: true},
		"uid":         schema.Int64Attribute{Optional: true, Computed: true},
		"gid":         schema.Int64Attribute{Optional: true, Computed: true},
		"params":      schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
		"groups":      schema.ListAttribute{Optional: true, ElementType: types.StringType},
		"format":      schema.StringAttribute{Optional: true},
	}}
}

func (r *pluginResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *pluginResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pluginModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating plugins.")
		return
	}
	payload, diags := pluginRequest(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result pluginResponse
	if err := r.client.Call("create_plugin", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps plugin failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.Plugin.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps plugin failed", "xyOps returned a plugin without an id.")
		return
	}
	resp.Diagnostics.Append(setPluginModel(ctx, &plan, result.Plugin)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pluginResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pluginModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading plugins.")
		return
	}
	var result pluginResponse
	err := r.client.Call("get_plugin", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps plugin failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setPluginModel(ctx, &state, result.Plugin)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *pluginResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state pluginModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating plugins.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{"id": state.ID.ValueString()}
	addChangedString(payload, "title", plan.Title, state.Title)
	addChangedString(payload, "type", plan.Type, state.Type)
	addChangedString(payload, "command", plan.Command, state.Command)
	addChangedString(payload, "format", plan.Format, state.Format)
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		payload["enabled"] = plan.Enabled.ValueBool()
	}
	if plan.ScriptFile.ValueString() != state.ScriptFile.ValueString() {
		payload["script"] = plan.ScriptFile.ValueString()
	}
	if plan.UID.ValueInt64() != state.UID.ValueInt64() {
		payload["uid"] = plan.UID.ValueInt64()
	}
	if plan.GID.ValueInt64() != state.GID.ValueInt64() {
		payload["gid"] = plan.GID.ValueInt64()
	}
	groups, gdiags := changedGroups(ctx, plan.Groups, state.Groups)
	resp.Diagnostics.Append(gdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if groups != nil {
		payload["groups"] = groups
	}
	params, pdiags, pchanged := jsonStringPayload(ctx, plan.Params, state.Params, "params")
	resp.Diagnostics.Append(pdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if pchanged {
		payload["params"] = params
	}
	if len(payload) > 1 {
		if err := r.client.Call("update_plugin", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps plugin failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pluginResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pluginModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting plugins.")
		return
	}
	if err := r.client.Call("delete_plugin", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps plugin failed", err.Error())
	}
}

func (r *pluginResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func pluginRequest(ctx context.Context, m pluginModel, withID bool) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{
		"title":   m.Title.ValueString(),
		"enabled": m.Enabled.ValueBool(),
		"type":    m.Type.ValueString(),
		"script":  m.ScriptFile.ValueString(),
	}
	if withID && !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Command.IsNull() && !m.Command.IsUnknown() {
		payload["command"] = m.Command.ValueString()
	}
	if !m.UID.IsNull() && !m.UID.IsUnknown() {
		payload["uid"] = m.UID.ValueInt64()
	}
	if !m.GID.IsNull() && !m.GID.IsUnknown() {
		payload["gid"] = m.GID.ValueInt64()
	}
	if !m.Format.IsNull() && !m.Format.IsUnknown() {
		payload["format"] = m.Format.ValueString()
	}
	if !m.Groups.IsNull() && !m.Groups.IsUnknown() {
		var groups []string
		var diags diag.Diagnostics
		diags.Append(m.Groups.ElementsAs(ctx, &groups, false)...)
		if diags.HasError() {
			return nil, diags
		}
		payload["groups"] = groups
		return withPluginParams(ctx, m, payload, diags)
	}
	var diags diag.Diagnostics
	return withPluginParams(ctx, m, payload, diags)
}

func withPluginParams(ctx context.Context, m pluginModel, payload map[string]any, diags diag.Diagnostics) (map[string]any, diag.Diagnostics) {
	if !m.Params.IsNull() && !m.Params.IsUnknown() {
		var params any
		if err := json.Unmarshal([]byte(m.Params.ValueString()), &params); err != nil {
			diags.AddAttributeError(path.Root("params"), "Invalid JSON", err.Error())
			return nil, diags
		}
		payload["params"] = params
	}
	return payload, diags
}

func setPluginModel(ctx context.Context, model *pluginModel, plugin pluginPayload) diag.Diagnostics {
	var diags diag.Diagnostics
	model.ID = types.StringValue(plugin.ID)
	model.Title = types.StringValue(plugin.Title)
	model.Enabled = types.BoolValue(plugin.Enabled)
	model.Type = types.StringValue(plugin.Type)
	model.ScriptFile = types.StringValue(plugin.Script)
	model.Command = stringValueOrNull(plugin.Command, model.Command)
	model.Format = stringValueOrNull(plugin.Format, model.Format)
	if plugin.UID != nil {
		model.UID = int64ValueOrNull(*plugin.UID, model.UID)
	} else {
		model.UID = types.Int64Null()
	}
	if plugin.GID != nil {
		model.GID = int64ValueOrNull(*plugin.GID, model.GID)
	} else {
		model.GID = types.Int64Null()
	}
	list, listDiags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(plugin.Groups))
	if listDiags.HasError() {
		return listDiags
	}
	model.Groups = list
	model.Params = jsonStringFromRawOptional(plugin.Params, model.Params)
	return diags
}
