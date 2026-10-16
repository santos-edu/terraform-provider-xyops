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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*alertResource)(nil)
	_ resource.ResourceWithConfigure   = (*alertResource)(nil)
	_ resource.ResourceWithImportState = (*alertResource)(nil)
)

type alertResource struct{ client *xyops.Client }

type alertModel struct {
	ID         types.String `tfsdk:"id"`
	Title      types.String `tfsdk:"title"`
	Expression types.String `tfsdk:"expression"`
	Message    types.String `tfsdk:"message"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Samples    types.Int64  `tfsdk:"samples"`
	MaxCount   types.Int64  `tfsdk:"max_count"`
	Notes      types.String `tfsdk:"notes"`
	MonitorID  types.String `tfsdk:"monitor_id"`
	Groups     types.List   `tfsdk:"groups"`
	Actions    types.String `tfsdk:"actions"`
}

type alertResponse struct {
	Alert alertPayload `json:"alert"`
}
type alertPayload struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Expression string          `json:"expression"`
	Message    string          `json:"message"`
	Enabled    bool            `json:"enabled"`
	Samples    int64           `json:"samples"`
	MaxCount   int64           `json:"max_count"`
	Notes      string          `json:"notes"`
	MonitorID  string          `json:"monitor_id"`
	Groups     []string        `json:"groups"`
	Actions    json.RawMessage `json:"actions"`
}

func NewAlertResource() resource.Resource { return &alertResource{} }

func (r *alertResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert"
}

func (r *alertResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":         schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":      schema.StringAttribute{Required: true},
		"expression": schema.StringAttribute{Required: true},
		"message":    schema.StringAttribute{Required: true},
		"enabled":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"samples":    schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1)},
		"max_count":  schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1)},
		"notes":      schema.StringAttribute{Optional: true},
		"monitor_id": schema.StringAttribute{Optional: true},
		"groups":     schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
		"actions":    schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
	}}
}

func (r *alertResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *alertResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan alertModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating alerts.")
		return
	}
	payload, diags := alertRequest(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result alertResponse
	if err := r.client.Call("create_alert", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps alert failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.Alert.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps alert failed", "xyOps returned an alert without an id.")
		return
	}
	resp.Diagnostics.Append(setAlertModel(ctx, &plan, result.Alert)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *alertResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state alertModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading alerts.")
		return
	}
	var result alertResponse
	err := r.client.Call("get_alert", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps alert failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setAlertModel(ctx, &state, result.Alert)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *alertResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state alertModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating alerts.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{"id": state.ID.ValueString()}
	addChangedString(payload, "title", plan.Title, state.Title)
	addChangedString(payload, "expression", plan.Expression, state.Expression)
	addChangedString(payload, "message", plan.Message, state.Message)
	addChangedString(payload, "notes", plan.Notes, state.Notes)
	addChangedString(payload, "monitor_id", plan.MonitorID, state.MonitorID)
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		payload["enabled"] = plan.Enabled.ValueBool()
	}
	if plan.Samples.ValueInt64() != state.Samples.ValueInt64() {
		payload["samples"] = plan.Samples.ValueInt64()
	}
	if plan.MaxCount.ValueInt64() != state.MaxCount.ValueInt64() {
		payload["max_count"] = plan.MaxCount.ValueInt64()
	}
	groups, gdiags := changedGroups(ctx, plan.Groups, state.Groups)
	resp.Diagnostics.Append(gdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if groups != nil {
		payload["groups"] = groups
	}
	actionsValue, actionsDiags, actionsChanged := jsonStringPayload(ctx, plan.Actions, state.Actions, "actions")
	resp.Diagnostics.Append(actionsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if actionsChanged {
		payload["actions"] = actionsValue
	}
	if len(payload) > 1 {
		if err := r.client.Call("update_alert", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps alert failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *alertResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state alertModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting alerts.")
		return
	}
	if err := r.client.Call("delete_alert", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps alert failed", err.Error())
	}
}

func (r *alertResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func alertRequest(ctx context.Context, m alertModel, withID bool) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{"title": m.Title.ValueString(), "expression": m.Expression.ValueString(), "message": m.Message.ValueString(), "enabled": m.Enabled.ValueBool(), "samples": m.Samples.ValueInt64(), "max_count": m.MaxCount.ValueInt64()}
	if withID && !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() {
		payload["notes"] = m.Notes.ValueString()
	}
	if !m.MonitorID.IsNull() && !m.MonitorID.IsUnknown() {
		payload["monitor_id"] = m.MonitorID.ValueString()
	}
	if !m.Groups.IsNull() && !m.Groups.IsUnknown() {
		var groups []string
		var diags diag.Diagnostics
		diags.Append(m.Groups.ElementsAs(ctx, &groups, false)...)
		if diags.HasError() {
			return nil, diags
		}
		payload["groups"] = groups
		return withActions(ctx, m, payload, diags)
	}
	var diags diag.Diagnostics
	return withActions(ctx, m, payload, diags)
}

func withActions(ctx context.Context, m alertModel, payload map[string]any, diags diag.Diagnostics) (map[string]any, diag.Diagnostics) {
	if !m.Actions.IsNull() && !m.Actions.IsUnknown() {
		var actions any
		if err := json.Unmarshal([]byte(m.Actions.ValueString()), &actions); err != nil {
			diags.AddAttributeError(path.Root("actions"), "Invalid JSON", err.Error())
			return nil, diags
		}
		payload["actions"] = actions
	}
	return payload, diags
}

func setAlertModel(ctx context.Context, model *alertModel, alert alertPayload) diag.Diagnostics {
	var diags diag.Diagnostics
	model.ID = types.StringValue(alert.ID)
	model.Title = types.StringValue(alert.Title)
	model.Expression = types.StringValue(alert.Expression)
	model.Message = types.StringValue(alert.Message)
	model.Enabled = types.BoolValue(alert.Enabled)
	model.Samples = types.Int64Value(alert.Samples)
	model.MaxCount = types.Int64Value(alert.MaxCount)
	model.Notes = stringValueOrNull(alert.Notes, model.Notes)
	model.MonitorID = stringValueOrNull(alert.MonitorID, model.MonitorID)
	list, listDiags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(alert.Groups))
	if listDiags.HasError() {
		return listDiags
	}
	model.Groups = list
	model.Actions = jsonStringFromRawOptional(alert.Actions, model.Actions)
	return diags
}
