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
	_ resource.Resource                = (*categoryResource)(nil)
	_ resource.ResourceWithConfigure   = (*categoryResource)(nil)
	_ resource.ResourceWithImportState = (*categoryResource)(nil)
)

type categoryResource struct{ client *xyops.Client }

type categoryModel struct {
	ID      types.String `tfsdk:"id"`
	Title   types.String `tfsdk:"title"`
	Enabled types.Bool   `tfsdk:"enabled"`
	Icon    types.String `tfsdk:"icon"`
	Color   types.String `tfsdk:"color"`
	Notes   types.String `tfsdk:"notes"`
	Actions types.String `tfsdk:"actions"`
	Limits  types.String `tfsdk:"limits"`
}

type categoryResponse struct {
	Category categoryPayload `json:"category"`
}

type categoryPayload struct {
	ID      string          `json:"id"`
	Title   string          `json:"title"`
	Enabled bool            `json:"enabled"`
	Icon    string          `json:"icon"`
	Color   string          `json:"color"`
	Notes   string          `json:"notes"`
	Actions json.RawMessage `json:"actions"`
	Limits  json.RawMessage `json:"limits"`
}

func NewCategoryResource() resource.Resource { return &categoryResource{} }

func (r *categoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_category"
}

func (r *categoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":      schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":   schema.StringAttribute{Required: true},
		"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"icon":    schema.StringAttribute{Optional: true},
		"color":   schema.StringAttribute{Optional: true},
		"notes":   schema.StringAttribute{Optional: true},
		"actions": schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
		"limits":  schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
	}}
}

func (r *categoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *categoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan categoryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating categories.")
		return
	}
	payload, diags := categoryRequest(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result categoryResponse
	if err := r.client.Call("create_category", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps category failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.Category.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps category failed", "xyOps returned a category without an id.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *categoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state categoryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading categories.")
		return
	}
	var result categoryResponse
	err := r.client.Call("get_category", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps category failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setCategoryModel(&state, result.Category)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *categoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state categoryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating categories.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{"id": state.ID.ValueString()}
	addChangedString(payload, "title", plan.Title, state.Title)
	addChangedString(payload, "icon", plan.Icon, state.Icon)
	addChangedString(payload, "color", plan.Color, state.Color)
	addChangedString(payload, "notes", plan.Notes, state.Notes)
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		payload["enabled"] = plan.Enabled.ValueBool()
	}
	actions, diags, changed := jsonStringPayload(ctx, plan.Actions, state.Actions, "actions")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if changed {
		payload["actions"] = actions
	}
	limits, ldiags, lchanged := jsonStringPayload(ctx, plan.Limits, state.Limits, "limits")
	resp.Diagnostics.Append(ldiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if lchanged {
		payload["limits"] = limits
	}
	if len(payload) > 1 {
		if err := r.client.Call("update_category", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps category failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *categoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state categoryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting categories.")
		return
	}
	if err := r.client.Call("delete_category", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps category failed", err.Error())
	}
}

func (r *categoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func categoryRequest(_ context.Context, m categoryModel, withID bool) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{"title": m.Title.ValueString(), "enabled": m.Enabled.ValueBool()}
	if withID && !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Icon.IsNull() && !m.Icon.IsUnknown() {
		payload["icon"] = m.Icon.ValueString()
	}
	if !m.Color.IsNull() && !m.Color.IsUnknown() {
		payload["color"] = m.Color.ValueString()
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() {
		payload["notes"] = m.Notes.ValueString()
	}
	var diags diag.Diagnostics
	if !m.Actions.IsNull() && !m.Actions.IsUnknown() {
		var actions any
		if err := json.Unmarshal([]byte(m.Actions.ValueString()), &actions); err != nil {
			diags.AddAttributeError(path.Root("actions"), "Invalid JSON", err.Error())
			return nil, diags
		}
		payload["actions"] = actions
	}
	if !m.Limits.IsNull() && !m.Limits.IsUnknown() {
		var limits any
		if err := json.Unmarshal([]byte(m.Limits.ValueString()), &limits); err != nil {
			diags.AddAttributeError(path.Root("limits"), "Invalid JSON", err.Error())
			return nil, diags
		}
		payload["limits"] = limits
	}
	return payload, diags
}

func setCategoryModel(model *categoryModel, category categoryPayload) diag.Diagnostics {
	model.ID = types.StringValue(category.ID)
	model.Title = types.StringValue(category.Title)
	model.Enabled = types.BoolValue(category.Enabled)
	model.Icon = stringValueOrNull(category.Icon, model.Icon)
	model.Color = stringValueOrNull(category.Color, model.Color)
	model.Notes = stringValueOrNull(category.Notes, model.Notes)
	model.Actions = jsonStringFromRawOptional(category.Actions, model.Actions)
	model.Limits = jsonStringFromRawOptional(category.Limits, model.Limits)
	return nil
}
