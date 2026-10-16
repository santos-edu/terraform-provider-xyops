package provider

import (
	"context"
	"errors"
	"fmt"
	"maps"

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
	_ resource.Resource                = (*roleResource)(nil)
	_ resource.ResourceWithConfigure   = (*roleResource)(nil)
	_ resource.ResourceWithImportState = (*roleResource)(nil)
)

type roleResource struct{ client *xyops.Client }

type roleModel struct {
	ID         types.String `tfsdk:"id"`
	Title      types.String `tfsdk:"title"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Privileges types.Map    `tfsdk:"privileges"`
	Categories types.List   `tfsdk:"categories"`
	Notes      types.String `tfsdk:"notes"`
}

type roleResponse struct {
	Role rolePayload `json:"role"`
}

type rolePayload struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Enabled    bool             `json:"enabled"`
	Privileges map[string]int64 `json:"privileges"`
	Categories []string         `json:"categories"`
	Notes      string           `json:"notes"`
}

func NewRoleResource() resource.Resource { return &roleResource{} }

func (r *roleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *roleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":         schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":      schema.StringAttribute{Required: true},
		"enabled":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"privileges": schema.MapAttribute{Optional: true, ElementType: types.Int64Type},
		"categories": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
		"notes":      schema.StringAttribute{Optional: true},
	}}
}

func (r *roleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *roleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating roles.")
		return
	}
	payload, diags := roleRequest(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result roleResponse
	if err := r.client.Call("create_role", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps role failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.Role.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps role failed", "xyOps returned a role without an id.")
		return
	}
	resp.Diagnostics.Append(setRoleModel(ctx, &plan, result.Role)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading roles.")
		return
	}
	var result roleResponse
	err := r.client.Call("get_role", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps role failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setRoleModel(ctx, &state, result.Role)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *roleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating roles.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{"id": state.ID.ValueString()}
	addChangedString(payload, "title", plan.Title, state.Title)
	addChangedString(payload, "notes", plan.Notes, state.Notes)
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		payload["enabled"] = plan.Enabled.ValueBool()
	}
	privileges, pdiags := changedInt64Map(ctx, plan.Privileges, state.Privileges)
	resp.Diagnostics.Append(pdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if privileges != nil {
		payload["privileges"] = privileges
	}
	categories, cdiags := changedGroups(ctx, plan.Categories, state.Categories)
	resp.Diagnostics.Append(cdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if categories != nil {
		payload["categories"] = categories
	}
	if len(payload) > 1 {
		if err := r.client.Call("update_role", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps role failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting roles.")
		return
	}
	if err := r.client.Call("delete_role", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps role failed", err.Error())
	}
}

func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func roleRequest(ctx context.Context, m roleModel, withID bool) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{"title": m.Title.ValueString(), "enabled": m.Enabled.ValueBool()}
	if withID && !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() {
		payload["notes"] = m.Notes.ValueString()
	}
	var diags diag.Diagnostics
	if !m.Privileges.IsNull() && !m.Privileges.IsUnknown() {
		var privileges map[string]int64
		diags.Append(m.Privileges.ElementsAs(ctx, &privileges, false)...)
		if diags.HasError() {
			return nil, diags
		}
		payload["privileges"] = privileges
	}
	if !m.Categories.IsNull() && !m.Categories.IsUnknown() {
		var categories []string
		diags.Append(m.Categories.ElementsAs(ctx, &categories, false)...)
		if diags.HasError() {
			return nil, diags
		}
		payload["categories"] = categories
	}
	return payload, diags
}

func setRoleModel(ctx context.Context, model *roleModel, role rolePayload) diag.Diagnostics {
	var diags diag.Diagnostics
	model.ID = types.StringValue(role.ID)
	model.Title = types.StringValue(role.Title)
	model.Enabled = types.BoolValue(role.Enabled)
	model.Notes = stringValueOrNull(role.Notes, model.Notes)
	if len(role.Privileges) > 0 || !model.Privileges.IsNull() {
		m, d := types.MapValueFrom(ctx, types.Int64Type, role.Privileges)
		if d.HasError() {
			return d
		}
		model.Privileges = m
	}
	l, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(role.Categories))
	if d.HasError() {
		return d
	}
	model.Categories = l
	return diags
}

func changedInt64Map(ctx context.Context, plan, state types.Map) (map[string]int64, diag.Diagnostics) {
	if plan.IsUnknown() {
		return nil, nil
	}
	var planMap, stateMap map[string]int64
	var diags diag.Diagnostics
	if !plan.IsNull() {
		diags.Append(plan.ElementsAs(ctx, &planMap, false)...)
		if diags.HasError() {
			return nil, diags
		}
	}
	if !state.IsNull() && !state.IsUnknown() {
		diags.Append(state.ElementsAs(ctx, &stateMap, false)...)
		if diags.HasError() {
			return nil, diags
		}
	}
	if maps.Equal(planMap, stateMap) {
		return nil, nil
	}
	return planMap, nil
}
