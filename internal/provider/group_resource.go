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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*groupResource)(nil)
	_ resource.ResourceWithConfigure   = (*groupResource)(nil)
	_ resource.ResourceWithImportState = (*groupResource)(nil)
)

type groupResource struct{ client *xyops.Client }

type groupModel struct {
	ID               types.String `tfsdk:"id"`
	Title            types.String `tfsdk:"title"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	HostnameMatch    types.String `tfsdk:"hostname_match"`
	SortOrder        types.Int64  `tfsdk:"sort_order"`
	AlertActions     types.String `tfsdk:"alert_actions"`
	Notes            types.String `tfsdk:"notes"`
	MaxJobsPerServer types.Int64  `tfsdk:"max_jobs_per_server"`
}

type groupResponse struct {
	Group groupPayload `json:"group"`
}

type groupPayload struct {
	ID               string          `json:"id"`
	Title            string          `json:"title"`
	Enabled          bool            `json:"enabled"`
	HostnameMatch    string          `json:"hostname_match"`
	SortOrder        int64           `json:"sort_order"`
	AlertActions     json.RawMessage `json:"alert_actions"`
	Notes            string          `json:"notes"`
	MaxJobsPerServer int64           `json:"max_jobs_per_server"`
}

func NewGroupResource() resource.Resource { return &groupResource{} }

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":                  schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":               schema.StringAttribute{Required: true},
		"enabled":             schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"hostname_match":      schema.StringAttribute{Required: true},
		"sort_order":          schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
		"alert_actions":       schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
		"notes":               schema.StringAttribute{Optional: true},
		"max_jobs_per_server": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0)},
	}}
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating groups.")
		return
	}
	payload, diags := groupRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result groupResponse
	if err := r.client.Call("create_group", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps group failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.Group.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps group failed", "xyOps returned a group without an id.")
		return
	}
	sortOrder := plan.SortOrder
	resp.Diagnostics.Append(setGroupModel(&plan, result.Group)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !sortOrder.IsNull() && !sortOrder.IsUnknown() {
		if err := r.client.Call("update_group", map[string]any{"id": plan.ID.ValueString(), "sort_order": sortOrder.ValueInt64()}, nil); err != nil {
			resp.Diagnostics.AddError("Set xyOps group sort_order failed", err.Error())
			return
		}
		plan.SortOrder = sortOrder
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading groups.")
		return
	}
	var result groupResponse
	err := r.client.Call("get_group", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps group failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setGroupModel(&state, result.Group)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating groups.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{"id": state.ID.ValueString()}
	addChangedString(payload, "title", plan.Title, state.Title)
	addChangedString(payload, "hostname_match", plan.HostnameMatch, state.HostnameMatch)
	addChangedString(payload, "notes", plan.Notes, state.Notes)
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		payload["enabled"] = plan.Enabled.ValueBool()
	}
	if plan.SortOrder.ValueInt64() != state.SortOrder.ValueInt64() {
		payload["sort_order"] = plan.SortOrder.ValueInt64()
	}
	if plan.MaxJobsPerServer.ValueInt64() != state.MaxJobsPerServer.ValueInt64() {
		payload["max_jobs_per_server"] = plan.MaxJobsPerServer.ValueInt64()
	}
	actions, diags, changed := jsonStringPayload(ctx, plan.AlertActions, state.AlertActions, "alert_actions")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if changed {
		payload["alert_actions"] = actions
	}
	if len(payload) > 1 {
		if err := r.client.Call("update_group", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps group failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting groups.")
		return
	}
	if err := r.client.Call("delete_group", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps group failed", err.Error())
	}
}

func (r *groupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func groupRequest(_ context.Context, m groupModel) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{
		"title":          m.Title.ValueString(),
		"enabled":        m.Enabled.ValueBool(),
		"hostname_match": m.HostnameMatch.ValueString(),
	}
	if !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() {
		payload["notes"] = m.Notes.ValueString()
	}
	if !m.MaxJobsPerServer.IsNull() && !m.MaxJobsPerServer.IsUnknown() {
		payload["max_jobs_per_server"] = m.MaxJobsPerServer.ValueInt64()
	}
	var diags diag.Diagnostics
	if !m.AlertActions.IsNull() && !m.AlertActions.IsUnknown() {
		var actions any
		if err := json.Unmarshal([]byte(m.AlertActions.ValueString()), &actions); err != nil {
			diags.AddAttributeError(path.Root("alert_actions"), "Invalid JSON", err.Error())
			return nil, diags
		}
		payload["alert_actions"] = actions
	}
	return payload, diags
}

func setGroupModel(model *groupModel, group groupPayload) diag.Diagnostics {
	model.ID = types.StringValue(group.ID)
	model.Title = types.StringValue(group.Title)
	model.Enabled = types.BoolValue(group.Enabled)
	model.HostnameMatch = types.StringValue(group.HostnameMatch)
	model.SortOrder = types.Int64Value(group.SortOrder)
	model.MaxJobsPerServer = types.Int64Value(group.MaxJobsPerServer)
	model.Notes = stringValueOrNull(group.Notes, model.Notes)
	model.AlertActions = jsonStringFromRawOptional(group.AlertActions, model.AlertActions)
	return nil
}
