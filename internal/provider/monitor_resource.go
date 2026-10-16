package provider

import (
	"context"
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
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*monitorResource)(nil)
	_ resource.ResourceWithConfigure   = (*monitorResource)(nil)
	_ resource.ResourceWithImportState = (*monitorResource)(nil)
)

type monitorResource struct{ client *xyops.Client }

type monitorModel struct {
	ID           types.String `tfsdk:"id"`
	Title        types.String `tfsdk:"title"`
	Source       types.String `tfsdk:"source"`
	DataType     types.String `tfsdk:"data_type"`
	Display      types.Bool   `tfsdk:"display"`
	Groups       types.List   `tfsdk:"groups"`
	Suffix       types.String `tfsdk:"suffix"`
	MinVertScale types.Int64  `tfsdk:"min_vert_scale"`
	MaxVertScale types.Int64  `tfsdk:"max_vert_scale"`
	DataMatch    types.String `tfsdk:"data_match"`
	Delta        types.Bool   `tfsdk:"delta"`
	DivideTime   types.Bool   `tfsdk:"divide_time"`
	ZeroMin      types.Int64  `tfsdk:"zero_min"`
	Notes        types.String `tfsdk:"notes"`
}

type monitorResponse struct {
	Monitor monitorPayload `json:"monitor"`
}

type monitorPayload struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Source       string   `json:"source"`
	DataType     string   `json:"data_type"`
	Display      bool     `json:"display"`
	Groups       []string `json:"groups"`
	Suffix       string   `json:"suffix"`
	MinVertScale int64    `json:"min_vert_scale"`
	MaxVertScale int64    `json:"max_vert_scale"`
	DataMatch    string   `json:"data_match"`
	Delta        bool     `json:"delta"`
	DivideTime   bool     `json:"divide_time"`
	ZeroMin      int64    `json:"zero_min"`
	Notes        string   `json:"notes"`
}

func NewMonitorResource() resource.Resource { return &monitorResource{} }

func (r *monitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *monitorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":             schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":          schema.StringAttribute{Required: true},
		"source":         schema.StringAttribute{Required: true},
		"data_type":      schema.StringAttribute{Required: true},
		"display":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"groups":         schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
		"suffix":         schema.StringAttribute{Optional: true},
		"min_vert_scale": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(1)},
		"max_vert_scale": schema.Int64Attribute{Optional: true, Computed: true},
		"data_match":     schema.StringAttribute{Optional: true, Computed: true},
		"delta":          schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		"divide_time":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		"zero_min":       schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0)},
		"notes":          schema.StringAttribute{Optional: true},
	}}
}

func (r *monitorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *monitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan monitorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating monitors.")
		return
	}
	payload, diags := monitorRequest(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result monitorResponse
	if err := r.client.Call("create_monitor", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps monitor failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.Monitor.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps monitor failed", "xyOps returned a monitor without an id.")
		return
	}
	resp.Diagnostics.Append(setMonitorModel(ctx, &plan, result.Monitor)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *monitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state monitorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading monitors.")
		return
	}
	var result monitorResponse
	err := r.client.Call("get_monitor", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps monitor failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setMonitorModel(ctx, &state, result.Monitor)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *monitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state monitorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating monitors.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{"id": state.ID.ValueString()}
	addChangedString(payload, "title", plan.Title, state.Title)
	addChangedString(payload, "source", plan.Source, state.Source)
	addChangedString(payload, "data_type", plan.DataType, state.DataType)
	addChangedString(payload, "suffix", plan.Suffix, state.Suffix)
	addChangedString(payload, "data_match", plan.DataMatch, state.DataMatch)
	addChangedString(payload, "notes", plan.Notes, state.Notes)
	if plan.Display.ValueBool() != state.Display.ValueBool() {
		payload["display"] = plan.Display.ValueBool()
	}
	if plan.MinVertScale.ValueInt64() != state.MinVertScale.ValueInt64() {
		payload["min_vert_scale"] = plan.MinVertScale.ValueInt64()
	}
	if plan.MaxVertScale.ValueInt64() != state.MaxVertScale.ValueInt64() {
		payload["max_vert_scale"] = plan.MaxVertScale.ValueInt64()
	}
	if plan.Delta.ValueBool() != state.Delta.ValueBool() {
		payload["delta"] = plan.Delta.ValueBool()
	}
	if plan.DivideTime.ValueBool() != state.DivideTime.ValueBool() {
		payload["divide_time"] = plan.DivideTime.ValueBool()
	}
	if plan.ZeroMin.ValueInt64() != state.ZeroMin.ValueInt64() {
		payload["zero_min"] = plan.ZeroMin.ValueInt64()
	}
	groups, gdiags := changedGroups(ctx, plan.Groups, state.Groups)
	resp.Diagnostics.Append(gdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if groups != nil {
		payload["groups"] = groups
	}
	if len(payload) > 1 {
		if err := r.client.Call("update_monitor", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps monitor failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *monitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state monitorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting monitors.")
		return
	}
	if err := r.client.Call("delete_monitor", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps monitor failed", err.Error())
	}
}

func (r *monitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func monitorRequest(ctx context.Context, m monitorModel, withID bool) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{
		"title":          m.Title.ValueString(),
		"source":         m.Source.ValueString(),
		"data_type":      m.DataType.ValueString(),
		"display":        m.Display.ValueBool(),
		"min_vert_scale": m.MinVertScale.ValueInt64(),
		"delta":          m.Delta.ValueBool(),
		"divide_time":    m.DivideTime.ValueBool(),
		"zero_min":       m.ZeroMin.ValueInt64(),
	}
	if withID && !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Suffix.IsNull() && !m.Suffix.IsUnknown() {
		payload["suffix"] = m.Suffix.ValueString()
	}
	if !m.DataMatch.IsNull() && !m.DataMatch.IsUnknown() {
		payload["data_match"] = m.DataMatch.ValueString()
	}
	if !m.MaxVertScale.IsNull() && !m.MaxVertScale.IsUnknown() {
		payload["max_vert_scale"] = m.MaxVertScale.ValueInt64()
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() {
		payload["notes"] = m.Notes.ValueString()
	}
	var diags diag.Diagnostics
	if !m.Groups.IsNull() && !m.Groups.IsUnknown() {
		var groups []string
		diags.Append(m.Groups.ElementsAs(ctx, &groups, false)...)
		if diags.HasError() {
			return nil, diags
		}
		payload["groups"] = groups
	}
	return payload, diags
}

func setMonitorModel(ctx context.Context, model *monitorModel, monitor monitorPayload) diag.Diagnostics {
	model.ID = types.StringValue(monitor.ID)
	model.Title = types.StringValue(monitor.Title)
	model.Source = types.StringValue(monitor.Source)
	model.DataType = types.StringValue(monitor.DataType)
	model.Display = types.BoolValue(monitor.Display)
	model.MinVertScale = types.Int64Value(monitor.MinVertScale)
	model.MaxVertScale = types.Int64Value(monitor.MaxVertScale)
	model.Delta = types.BoolValue(monitor.Delta)
	model.DivideTime = types.BoolValue(monitor.DivideTime)
	model.ZeroMin = types.Int64Value(monitor.ZeroMin)
	model.Suffix = stringValueOrNull(monitor.Suffix, model.Suffix)
	model.DataMatch = stringValueOrNull(monitor.DataMatch, model.DataMatch)
	model.Notes = stringValueOrNull(monitor.Notes, model.Notes)
	list, listDiags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(monitor.Groups))
	if listDiags.HasError() {
		return listDiags
	}
	model.Groups = list
	return nil
}
