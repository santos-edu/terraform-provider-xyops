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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*serverResource)(nil)
	_ resource.ResourceWithConfigure   = (*serverResource)(nil)
	_ resource.ResourceWithImportState = (*serverResource)(nil)
)

type serverResource struct{ client *xyops.Client }

type serverModel struct {
	ID        types.String `tfsdk:"id"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	Notes     types.String `tfsdk:"notes"`
	Hostname  types.String `tfsdk:"hostname"`
	IP        types.String `tfsdk:"ip"`
	AutoGroup types.Bool   `tfsdk:"auto_group"`
	Groups    types.List   `tfsdk:"groups"`
	Created   types.Int64  `tfsdk:"created"`
	Modified  types.Int64  `tfsdk:"modified"`
}

type serverResponse struct {
	Server serverPayload `json:"server"`
}

type serverPayload struct {
	ID        string   `json:"id"`
	Hostname  string   `json:"hostname"`
	Enabled   bool     `json:"enabled"`
	Notes     string   `json:"notes"`
	Groups    []string `json:"groups"`
	AutoGroup bool     `json:"autoGroup"`
	IP        string   `json:"ip"`
	Created   int64    `json:"created"`
	Modified  int64    `json:"modified"`
}

func NewServerResource() resource.Resource { return &serverResource{} }

func (r *serverResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

func (r *serverResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Required:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			Validators: []validator.String{newRegexStringValidator(
				`^[a-z0-9_]+$`,
				"The value must be a valid xyOps server id: lowercase letters, digits and underscores only.",
			)},
		},
		"enabled":    schema.BoolAttribute{Optional: true, Computed: true},
		"notes":      schema.StringAttribute{Optional: true, Computed: true},
		"hostname":   schema.StringAttribute{Computed: true},
		"ip":         schema.StringAttribute{Computed: true},
		"auto_group": schema.BoolAttribute{Computed: true},
		"groups":     schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"created":    schema.Int64Attribute{Computed: true},
		"modified":   schema.Int64Attribute{Computed: true},
	}}
}

func (r *serverResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serverResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before adopting servers.")
		return
	}
	if plan.ID.IsUnknown() {
		resp.Diagnostics.AddError(
			"Unknown xyOps server id",
			"xyops_server adopts a server that already exists, so its id must be known before the apply. Resolve the value the id comes from, a variable or another resource for example, and apply again.",
		)
		return
	}
	id := plan.ID.ValueString()
	server, diags := r.lookupServer(id)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if payload := adoptionPayload(id, plan, server); len(payload) > 1 {
		if err := r.client.Call("update_server", payload, nil); err != nil {
			resp.Diagnostics.AddError("Adopt xyOps server failed", err.Error())
			return
		}
		server, diags = r.verifyServerWrite(id, plan)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
	}
	// The snapshot is the source of truth, never the plan: writing a Computed
	// attribute the API did not confirm turns a silent no-op into state that
	// does not match the server.
	resp.Diagnostics.Append(setServerModel(ctx, &plan, server)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serverResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading servers.")
		return
	}
	var result serverResponse
	err := r.client.Call("get_server", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		summary, detail := serverLookupError(state.ID.ValueString(), err)
		resp.Diagnostics.AddError(summary, detail)
		return
	}
	resp.Diagnostics.Append(setServerModel(ctx, &state, result.Server)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state serverModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating servers.")
		return
	}
	id := state.ID.ValueString()
	payload := map[string]any{"id": id}
	if !plan.Enabled.IsUnknown() && plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		payload["enabled"] = plan.Enabled.ValueBool()
	}
	addChangedString(payload, "notes", plan.Notes, state.Notes)
	if len(payload) > 1 {
		if err := r.client.Call("update_server", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps server failed", err.Error())
			return
		}
		server, diags := r.verifyServerWrite(id, plan)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		resp.Diagnostics.Append(setServerModel(ctx, &plan, server)...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}
	// An omitted Optional+Computed attribute comes back unknown in the plan,
	// which Terraform rejects after apply. The API was left untouched, so the
	// previous read carries the values, including enabled and notes: both are
	// Optional+Computed without a default, so both can be unknown.
	if plan.Enabled.IsUnknown() {
		plan.Enabled = state.Enabled
	}
	if plan.Notes.IsUnknown() {
		plan.Notes = state.Notes
	}
	plan.Hostname = state.Hostname
	plan.IP = state.IP
	plan.AutoGroup = state.AutoGroup
	plan.Groups = state.Groups
	plan.Created = state.Created
	plan.Modified = state.Modified
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete calls no xyOps action. delete_server does not remove the record: it
// only evicts the server from the live registry, after which update_server
// keeps accepting writes that no read ever shows, so the server becomes
// permanently unmanageable. Restoring enabled = true on destroy would also
// surprise the operator, because the original value may have been false.
// Dropping the resource block and running destroy therefore stop managing
// the server and change nothing.
func (r *serverResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.AddWarning(
		"xyOps server left unmanaged",
		fmt.Sprintf("xyOps does not allow deleting servers through its API, so Terraform only stopped managing %q. The server and its current settings were left untouched. To stop managing it without running destroy, remove the resource block from the configuration instead.", state.ID.ValueString()),
	)
}

func (r *serverResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// adoptionPayload sends only the managed attributes the configuration asked
// for and the server does not already have. An unused write still advances the
// server modified timestamp, so it is not free. enabled carries no Default on
// purpose: one would re-enable a server the operator had deliberately disabled.
// Omitting it adopts the current value and writes nothing, at the cost of
// Optional+Computed not telling "I do not care" from "I forgot".
func adoptionPayload(id string, plan serverModel, server serverPayload) map[string]any {
	payload := map[string]any{"id": id}
	if !plan.Enabled.IsUnknown() && plan.Enabled.ValueBool() != server.Enabled {
		payload["enabled"] = plan.Enabled.ValueBool()
	}
	if !plan.Notes.IsUnknown() && plan.Notes.ValueString() != server.Notes {
		payload["notes"] = plan.Notes.ValueString()
	}
	return payload
}

func (r *serverResource) lookupServer(id string) (serverPayload, diag.Diagnostics) {
	var diags diag.Diagnostics
	var result serverResponse
	if err := r.client.Call("get_server", map[string]any{"id": id}, &result); err != nil {
		summary, detail := serverLookupError(id, err)
		diags.AddError(summary, detail)
		return serverPayload{}, diags
	}
	return result.Server, diags
}

// verifyServerWrite reads the server back after a write. update_server answers
// code 0 without returning the object, so the read is the only way to know the
// write landed, and failing loudly beats recording state xyOps does not have.
func (r *serverResource) verifyServerWrite(id string, plan serverModel) (serverPayload, diag.Diagnostics) {
	server, diags := r.lookupServer(id)
	if diags.HasError() {
		return serverPayload{}, diags
	}
	if !plan.Enabled.IsUnknown() && plan.Enabled.ValueBool() != server.Enabled {
		diags.AddError(
			"xyOps server write was not applied",
			fmt.Sprintf("xyOps accepted the update of server %q but the follow-up read reports enabled = %t while the configuration asks for %t.", id, server.Enabled, plan.Enabled.ValueBool()),
		)
	}
	if !plan.Notes.IsUnknown() && plan.Notes.ValueString() != server.Notes {
		diags.AddError(
			"xyOps server write was not applied",
			fmt.Sprintf("xyOps accepted the update of server %q but the follow-up read reports notes = %q while the configuration asks for %q.", id, server.Notes, plan.Notes.ValueString()),
		)
	}
	if diags.HasError() {
		return serverPayload{}, diags
	}
	return server, diags
}

// serverLookupError tells apart the two failures get_server reports for an id
// it will not serve. "notfound" means no server carries that id, while code
// "api" means xyOps refused the shape of the id itself and never looked it up.
func serverLookupError(id string, err error) (summary, detail string) {
	var apiErr *xyops.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.IsNotFound():
			return "xyOps server does not exist", fmt.Sprintf(
				"Server %q does not exist in xyOps. Servers register themselves the first time their agent contacts xyOps, so this resource can only adopt a server xyOps already knows about. Confirm the id in the xyOps UI.",
				id,
			)
		case apiErr.Code == "api":
			return "Invalid xyOps server id", fmt.Sprintf(
				"xyOps rejected the server id %q: %s. Server ids only accept lowercase letters, digits and underscores.",
				id, apiErr.Description,
			)
		}
	}
	return "Read xyOps server failed", err.Error()
}

func setServerModel(ctx context.Context, model *serverModel, server serverPayload) diag.Diagnostics {
	var diags diag.Diagnostics
	model.ID = types.StringValue(server.ID)
	model.Enabled = types.BoolValue(server.Enabled)
	model.Notes = stringValueOrNull(server.Notes, model.Notes)
	model.Hostname = types.StringValue(server.Hostname)
	model.IP = types.StringValue(server.IP)
	model.AutoGroup = types.BoolValue(server.AutoGroup)
	model.Created = types.Int64Value(server.Created)
	model.Modified = types.Int64Value(server.Modified)
	groups, groupDiags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(server.Groups))
	if groupDiags.HasError() {
		return groupDiags
	}
	model.Groups = groups
	return diags
}
