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
	_ resource.Resource                = (*channelResource)(nil)
	_ resource.ResourceWithConfigure   = (*channelResource)(nil)
	_ resource.ResourceWithImportState = (*channelResource)(nil)
)

type channelResource struct{ client *xyops.Client }

type channelModel struct {
	ID       types.String `tfsdk:"id"`
	Title    types.String `tfsdk:"title"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	Notes    types.String `tfsdk:"notes"`
	Users    types.List   `tfsdk:"users"`
	Email    types.String `tfsdk:"email"`
	WebHook  types.String `tfsdk:"web_hook"`
	RunEvent types.String `tfsdk:"run_event"`
	Sound    types.String `tfsdk:"sound"`
	Icon     types.String `tfsdk:"icon"`
}

type channelResponse struct {
	Channel channelPayload `json:"channel"`
}

type channelPayload struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Enabled  bool     `json:"enabled"`
	Notes    string   `json:"notes"`
	Users    []string `json:"users"`
	Email    string   `json:"email"`
	WebHook  string   `json:"web_hook"`
	RunEvent string   `json:"run_event"`
	Sound    string   `json:"sound"`
	Icon     string   `json:"icon"`
}

func NewChannelResource() resource.Resource { return &channelResource{} }

func (r *channelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_channel"
}

func (r *channelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":        schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":     schema.StringAttribute{Required: true},
		"enabled":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"notes":     schema.StringAttribute{Optional: true},
		"users":     schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
		"email":     schema.StringAttribute{Optional: true},
		"web_hook":  schema.StringAttribute{Optional: true},
		"run_event": schema.StringAttribute{Optional: true},
		"sound":     schema.StringAttribute{Optional: true},
		"icon":      schema.StringAttribute{Optional: true},
	}}
}

func (r *channelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *channelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan channelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating channels.")
		return
	}
	payload, diags := channelRequest(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result channelResponse
	if err := r.client.Call("create_channel", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps channel failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.Channel.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps channel failed", "xyOps returned a channel without an id.")
		return
	}
	resp.Diagnostics.Append(setChannelModel(ctx, &plan, result.Channel)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *channelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state channelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading channels.")
		return
	}
	var result channelResponse
	err := r.client.Call("get_channel", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps channel failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setChannelModel(ctx, &state, result.Channel)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *channelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state channelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating channels.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{"id": state.ID.ValueString()}
	addChangedString(payload, "title", plan.Title, state.Title)
	addChangedString(payload, "notes", plan.Notes, state.Notes)
	addChangedString(payload, "email", plan.Email, state.Email)
	addChangedString(payload, "web_hook", plan.WebHook, state.WebHook)
	addChangedString(payload, "run_event", plan.RunEvent, state.RunEvent)
	addChangedString(payload, "sound", plan.Sound, state.Sound)
	addChangedString(payload, "icon", plan.Icon, state.Icon)
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		payload["enabled"] = plan.Enabled.ValueBool()
	}
	users, udiags := changedGroups(ctx, plan.Users, state.Users)
	resp.Diagnostics.Append(udiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if users != nil {
		payload["users"] = users
	}
	if len(payload) > 1 {
		if err := r.client.Call("update_channel", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps channel failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *channelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state channelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting channels.")
		return
	}
	if err := r.client.Call("delete_channel", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps channel failed", err.Error())
	}
}

func (r *channelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func channelRequest(ctx context.Context, m channelModel, withID bool) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{"title": m.Title.ValueString(), "enabled": m.Enabled.ValueBool()}
	if withID && !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() {
		payload["notes"] = m.Notes.ValueString()
	}
	if !m.Email.IsNull() && !m.Email.IsUnknown() {
		payload["email"] = m.Email.ValueString()
	}
	if !m.WebHook.IsNull() && !m.WebHook.IsUnknown() {
		payload["web_hook"] = m.WebHook.ValueString()
	}
	if !m.RunEvent.IsNull() && !m.RunEvent.IsUnknown() {
		payload["run_event"] = m.RunEvent.ValueString()
	}
	if !m.Sound.IsNull() && !m.Sound.IsUnknown() {
		payload["sound"] = m.Sound.ValueString()
	}
	if !m.Icon.IsNull() && !m.Icon.IsUnknown() {
		payload["icon"] = m.Icon.ValueString()
	}
	var diags diag.Diagnostics
	if !m.Users.IsNull() && !m.Users.IsUnknown() {
		var users []string
		diags.Append(m.Users.ElementsAs(ctx, &users, false)...)
		if diags.HasError() {
			return nil, diags
		}
		payload["users"] = users
	}
	return payload, diags
}

func setChannelModel(ctx context.Context, model *channelModel, channel channelPayload) diag.Diagnostics {
	var diags diag.Diagnostics
	model.ID = types.StringValue(channel.ID)
	model.Title = types.StringValue(channel.Title)
	model.Enabled = types.BoolValue(channel.Enabled)
	model.Notes = stringValueOrNull(channel.Notes, model.Notes)
	model.Email = stringValueOrNull(channel.Email, model.Email)
	model.WebHook = stringValueOrNull(channel.WebHook, model.WebHook)
	model.RunEvent = stringValueOrNull(channel.RunEvent, model.RunEvent)
	model.Sound = stringValueOrNull(channel.Sound, model.Sound)
	model.Icon = stringValueOrNull(channel.Icon, model.Icon)
	list, listDiags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(channel.Users))
	if listDiags.HasError() {
		return listDiags
	}
	model.Users = list
	return diags
}

// nonNilStrings keeps the order the API returned and guarantees a known empty
// list, because Terraform rejects unknown or null values for a computed list.
func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
