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
	_ resource.Resource                = (*secretResource)(nil)
	_ resource.ResourceWithConfigure   = (*secretResource)(nil)
	_ resource.ResourceWithImportState = (*secretResource)(nil)
)

type secretResource struct{ client *xyops.Client }

type secretModel struct {
	ID       types.String `tfsdk:"id"`
	Title    types.String `tfsdk:"title"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	Notes    types.String `tfsdk:"notes"`
	Value    types.String `tfsdk:"value"`
	Category types.List   `tfsdk:"category"`
	Params   types.String `tfsdk:"params"`
	Icon     types.String `tfsdk:"icon"`
}

type secretResponse struct {
	Secret secretPayload `json:"secret"`
}

type secretPayload struct {
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Enabled  bool            `json:"enabled"`
	Notes    string          `json:"notes"`
	Value    string          `json:"value"`
	Category []string        `json:"category"`
	Params   json.RawMessage `json:"params"`
	Icon     string          `json:"icon"`
}

func NewSecretResource() resource.Resource { return &secretResource{} }

func (r *secretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

func (r *secretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":       schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":    schema.StringAttribute{Required: true},
		"enabled":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"notes":    schema.StringAttribute{Optional: true},
		"value":    schema.StringAttribute{Required: true, Sensitive: true},
		"category": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
		"params":   schema.StringAttribute{Optional: true, Computed: true, Validators: []validator.String{jsonStringValidator{}}},
		"icon":     schema.StringAttribute{Optional: true},
	}}
}

func (r *secretResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *secretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan secretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating secrets.")
		return
	}
	payload, diags := secretRequest(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result secretResponse
	if err := r.client.Call("create_secret", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps secret failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.Secret.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps secret failed", "xyOps returned a secret without an id.")
		return
	}
	resp.Diagnostics.Append(setSecretModel(ctx, &plan, result.Secret)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *secretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state secretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading secrets.")
		return
	}
	var result secretResponse
	err := r.client.Call("get_secret", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps secret failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setSecretModel(ctx, &state, result.Secret)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *secretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state secretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating secrets.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{"id": state.ID.ValueString()}
	addChangedString(payload, "title", plan.Title, state.Title)
	addChangedString(payload, "notes", plan.Notes, state.Notes)
	addChangedString(payload, "value", plan.Value, state.Value)
	addChangedString(payload, "icon", plan.Icon, state.Icon)
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		payload["enabled"] = plan.Enabled.ValueBool()
	}
	category, cdiags := changedGroups(ctx, plan.Category, state.Category)
	resp.Diagnostics.Append(cdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if category != nil {
		payload["category"] = category
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
		if err := r.client.Call("update_secret", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps secret failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *secretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state secretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting secrets.")
		return
	}
	if err := r.client.Call("delete_secret", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps secret failed", err.Error())
	}
}

func (r *secretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func secretRequest(ctx context.Context, m secretModel, withID bool) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{"title": m.Title.ValueString(), "enabled": m.Enabled.ValueBool(), "value": m.Value.ValueString()}
	if withID && !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() {
		payload["notes"] = m.Notes.ValueString()
	}
	if !m.Icon.IsNull() && !m.Icon.IsUnknown() {
		payload["icon"] = m.Icon.ValueString()
	}
	var diags diag.Diagnostics
	if !m.Category.IsNull() && !m.Category.IsUnknown() {
		var category []string
		diags.Append(m.Category.ElementsAs(ctx, &category, false)...)
		if diags.HasError() {
			return nil, diags
		}
		payload["category"] = category
	}
	diags.Append(addJSONToPayloadCreate(payload, "params", m.Params, []any{})...)
	return payload, diags
}

func setSecretModel(ctx context.Context, model *secretModel, secret secretPayload) diag.Diagnostics {
	var diags diag.Diagnostics
	model.ID = types.StringValue(secret.ID)
	model.Title = types.StringValue(secret.Title)
	model.Enabled = types.BoolValue(secret.Enabled)
	model.Notes = stringValueOrNull(secret.Notes, model.Notes)
	model.Value = types.StringValue(secret.Value)
	model.Icon = stringValueOrNull(secret.Icon, model.Icon)
	model.Params = jsonStringFromRaw(secret.Params, model.Params)
	list, listDiags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(secret.Category))
	if listDiags.HasError() {
		return listDiags
	}
	model.Category = list
	return diags
}
