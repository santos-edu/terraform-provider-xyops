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
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*apiKeyResource)(nil)
	_ resource.ResourceWithConfigure   = (*apiKeyResource)(nil)
	_ resource.ResourceWithImportState = (*apiKeyResource)(nil)
)

type apiKeyResource struct{ client *xyops.Client }

type apiKeyModel struct {
	ID          types.String `tfsdk:"id"`
	Title       types.String `tfsdk:"title"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Expires     types.Int64  `tfsdk:"expires"`
	Key         types.String `tfsdk:"key"`
	PlainKey    types.String `tfsdk:"plain_key"`
	Privileges  types.Map    `tfsdk:"privileges"`
	Roles       types.List   `tfsdk:"roles"`
}

type apiKeyCreateResponse struct {
	APIKey   apiKeyPayload `json:"api_key"`
	PlainKey string        `json:"plain_key"`
}

type apiKeyResponse struct {
	APIKey apiKeyPayload `json:"api_key"`
}

type apiKeyPayload struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Active      json.RawMessage  `json:"active"`
	Expires     *int64           `json:"expires"`
	Key         string           `json:"key"`
	Privileges  map[string]int64 `json:"privileges"`
	Roles       []string         `json:"roles"`
}

func NewAPIKeyResource() resource.Resource { return &apiKeyResource{} }

func (r *apiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *apiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":          schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":       schema.StringAttribute{Required: true},
		"description": schema.StringAttribute{Optional: true},
		"active":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"expires":     schema.Int64Attribute{Optional: true},
		"key":         schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"plain_key":   schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"privileges":  schema.MapAttribute{Optional: true, ElementType: types.Int64Type},
		"roles":       schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
	}}
}

func (r *apiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating API keys.")
		return
	}
	payload, diags := apiKeyRequest(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result apiKeyCreateResponse
	if err := r.client.Call("create_api_key", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps API key failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.APIKey.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps API key failed", "xyOps returned an API key without an id.")
		return
	}
	resp.Diagnostics.Append(setAPIKeyModel(ctx, &plan, result.APIKey)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// key and plain_key only come back from create, never from a get, so the
	// create response is the only source for them.
	plan.Key = types.StringValue(result.APIKey.Key)
	plan.PlainKey = types.StringValue(result.PlainKey)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading API keys.")
		return
	}
	var result apiKeyResponse
	err := r.client.Call("get_api_key", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps API key failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setAPIKeyModel(ctx, &state, result.APIKey)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state apiKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating API keys.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{"id": state.ID.ValueString()}
	addChangedString(payload, "title", plan.Title, state.Title)
	addChangedString(payload, "description", plan.Description, state.Description)
	if plan.Active.ValueBool() != state.Active.ValueBool() {
		payload["active"] = plan.Active.ValueBool()
	}
	if !plan.Expires.IsUnknown() && !plan.Expires.Equal(state.Expires) {
		if plan.Expires.IsNull() {
			payload["expires"] = 0
		} else {
			payload["expires"] = plan.Expires.ValueInt64()
		}
	}
	privileges, pdiags := changedInt64Map(ctx, plan.Privileges, state.Privileges)
	resp.Diagnostics.Append(pdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if privileges != nil {
		payload["privileges"] = privileges
	}
	roles, rdiags := changedGroups(ctx, plan.Roles, state.Roles)
	resp.Diagnostics.Append(rdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if roles != nil {
		payload["roles"] = roles
	}
	if len(payload) > 1 {
		if err := r.client.Call("update_api_key", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps API key failed", err.Error())
			return
		}
	}
	plan.Key = state.Key
	plan.PlainKey = state.PlainKey
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting API keys.")
		return
	}
	if err := r.client.Call("delete_api_key", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps API key failed", err.Error())
	}
}

func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func apiKeyRequest(ctx context.Context, m apiKeyModel, withID bool) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{"title": m.Title.ValueString(), "active": m.Active.ValueBool()}
	if withID && !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		payload["description"] = m.Description.ValueString()
	}
	if !m.Expires.IsNull() && !m.Expires.IsUnknown() {
		payload["expires"] = m.Expires.ValueInt64()
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
	if !m.Roles.IsNull() && !m.Roles.IsUnknown() {
		var roles []string
		diags.Append(m.Roles.ElementsAs(ctx, &roles, false)...)
		if diags.HasError() {
			return nil, diags
		}
		payload["roles"] = roles
	}
	return payload, diags
}

func setAPIKeyModel(ctx context.Context, model *apiKeyModel, key apiKeyPayload) diag.Diagnostics {
	var diags diag.Diagnostics
	model.ID = types.StringValue(key.ID)
	model.Title = types.StringValue(key.Title)
	model.Active = types.BoolValue(boolFromJSON(key.Active))
	model.Description = stringValueOrNull(key.Description, model.Description)
	var expires int64
	if key.Expires != nil {
		expires = *key.Expires
	}
	model.Expires = int64ValueOrNull(expires, model.Expires)
	if key.Key != "" {
		model.Key = types.StringValue(key.Key)
	}
	if len(key.Privileges) > 0 || !model.Privileges.IsNull() {
		m, d := types.MapValueFrom(ctx, types.Int64Type, key.Privileges)
		if d.HasError() {
			return d
		}
		model.Privileges = m
	}
	l, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(key.Roles))
	if d.HasError() {
		return d
	}
	model.Roles = l
	return diags
}

func boolFromJSON(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n != 0
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s == "true" || s == "1" || s == "yes"
	}
	return false
}
