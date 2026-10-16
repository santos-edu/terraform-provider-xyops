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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*tagResource)(nil)
	_ resource.ResourceWithConfigure   = (*tagResource)(nil)
	_ resource.ResourceWithImportState = (*tagResource)(nil)
)

type tagResource struct{ client *xyops.Client }

type tagModel struct {
	ID      types.String `tfsdk:"id"`
	Title   types.String `tfsdk:"title"`
	Enabled types.Bool   `tfsdk:"enabled"`
	Icon    types.String `tfsdk:"icon"`
	Color   types.String `tfsdk:"color"`
	Notes   types.String `tfsdk:"notes"`
}

type tagResponse struct {
	Tag tagPayload `json:"tag"`
}

type tagPayload struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Enabled bool   `json:"enabled"`
	Icon    string `json:"icon"`
	Color   string `json:"color"`
	Notes   string `json:"notes"`
}

func NewTagResource() resource.Resource { return &tagResource{} }

func (r *tagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag"
}

func (r *tagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":      schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":   schema.StringAttribute{Required: true},
		"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"icon":    schema.StringAttribute{Optional: true},
		"color":   schema.StringAttribute{Optional: true},
		"notes":   schema.StringAttribute{Optional: true},
	}}
}

func (r *tagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *tagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tagModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating tags.")
		return
	}
	payload := tagRequest(plan, true)
	var result tagResponse
	if err := r.client.Call("create_tag", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps tag failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.Tag.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps tag failed", "xyOps returned a tag without an id.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tagModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading tags.")
		return
	}
	var result tagResponse
	err := r.client.Call("get_tag", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps tag failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setTagModel(&state, result.Tag)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *tagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state tagModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating tags.")
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
	if len(payload) > 1 {
		if err := r.client.Call("update_tag", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps tag failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tagModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting tags.")
		return
	}
	if err := r.client.Call("delete_tag", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps tag failed", err.Error())
	}
}

func (r *tagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func tagRequest(m tagModel, withID bool) map[string]any {
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
	return payload
}

func setTagModel(model *tagModel, tag tagPayload) diag.Diagnostics {
	model.ID = types.StringValue(tag.ID)
	model.Title = types.StringValue(tag.Title)
	model.Enabled = types.BoolValue(tag.Enabled)
	model.Icon = stringValueOrNull(tag.Icon, model.Icon)
	model.Color = stringValueOrNull(tag.Color, model.Color)
	model.Notes = stringValueOrNull(tag.Notes, model.Notes)
	return nil
}
