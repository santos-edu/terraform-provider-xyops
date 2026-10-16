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
	_ resource.Resource                = (*eventResource)(nil)
	_ resource.ResourceWithConfigure   = (*eventResource)(nil)
	_ resource.ResourceWithImportState = (*eventResource)(nil)
)

type eventResource struct{ client *xyops.Client }

type eventModel struct {
	ID       types.String `tfsdk:"id"`
	Title    types.String `tfsdk:"title"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	Category types.String `tfsdk:"category"`
	Plugin   types.String `tfsdk:"plugin"`
	Targets  types.List   `tfsdk:"targets"`
	Params   types.String `tfsdk:"params"`
	Fields   types.String `tfsdk:"fields"`
	Tags     types.List   `tfsdk:"tags"`
	Algo     types.String `tfsdk:"algo"`
	Notes    types.String `tfsdk:"notes"`
	Actions  types.String `tfsdk:"actions"`
	Limits   types.String `tfsdk:"limits"`
	Triggers types.String `tfsdk:"triggers"`
}

type eventResponse struct {
	Event eventPayload `json:"event"`
}

type eventPayload struct {
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Enabled  bool            `json:"enabled"`
	Category string          `json:"category"`
	Plugin   string          `json:"plugin"`
	Targets  []string        `json:"targets"`
	Params   json.RawMessage `json:"params"`
	Fields   json.RawMessage `json:"fields"`
	Tags     []string        `json:"tags"`
	Algo     string          `json:"algo"`
	Notes    string          `json:"notes"`
	Actions  json.RawMessage `json:"actions"`
	Limits   json.RawMessage `json:"limits"`
	Triggers json.RawMessage `json:"triggers"`
	Workflow json.RawMessage `json:"workflow"`
	Type     string          `json:"type"`
}

func NewEventResource() resource.Resource { return &eventResource{} }

func (r *eventResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_event"
}

func (r *eventResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":       schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":    schema.StringAttribute{Required: true},
		"enabled":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"category": schema.StringAttribute{Required: true},
		"plugin":   schema.StringAttribute{Required: true},
		"targets":  schema.ListAttribute{Required: true, ElementType: types.StringType},
		"params":   schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
		"fields":   schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
		"tags":     schema.ListAttribute{Optional: true, ElementType: types.StringType},
		"algo":     schema.StringAttribute{Optional: true},
		"notes":    schema.StringAttribute{Optional: true},
		"actions":  schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
		"limits":   schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
		"triggers": schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
	}}
}

func (r *eventResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *eventResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan eventModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating events.")
		return
	}
	payload, diags := eventRequest(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result eventResponse
	if err := r.client.Call("create_event", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps event failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.Event.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps event failed", "xyOps returned an event without an id.")
		return
	}
	// The lists come from the response because the API echoes them back and the
	// state must carry the order it will read again. The JSON attributes stay
	// exactly as the plan left them: they are Optional without Computed, so
	// omitting them means null and adopting the empty container the API returns
	// would show a config diff. xyops_workflow keeps its non-computed JSON the
	// same way.
	targets, targetsDiags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(result.Event.Targets))
	resp.Diagnostics.Append(targetsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Targets = targets
	if len(result.Event.Tags) > 0 || !plan.Tags.IsNull() {
		tags, tagsDiags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(result.Event.Tags))
		resp.Diagnostics.Append(tagsDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		plan.Tags = tags
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *eventResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state eventModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading events.")
		return
	}
	var result eventResponse
	err := r.client.Call("get_event", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps event failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setEventModel(ctx, &state, result.Event)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *eventResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state eventModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating events.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{"id": state.ID.ValueString()}
	addChangedString(payload, "title", plan.Title, state.Title)
	addChangedString(payload, "category", plan.Category, state.Category)
	addChangedString(payload, "plugin", plan.Plugin, state.Plugin)
	addChangedString(payload, "algo", plan.Algo, state.Algo)
	addChangedString(payload, "notes", plan.Notes, state.Notes)
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		payload["enabled"] = plan.Enabled.ValueBool()
	}
	targets, tdiags := changedGroups(ctx, plan.Targets, state.Targets)
	resp.Diagnostics.Append(tdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if targets != nil {
		payload["targets"] = targets
	}
	tags, gdiags := changedGroups(ctx, plan.Tags, state.Tags)
	resp.Diagnostics.Append(gdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if tags != nil {
		payload["tags"] = tags
	}
	paramsValue, paramsDiags, paramsChanged := jsonStringPayload(ctx, plan.Params, state.Params, "params")
	resp.Diagnostics.Append(paramsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if paramsChanged {
		payload["params"] = paramsValue
	}
	fieldsValue, fieldsDiags, fieldsChanged := jsonStringPayload(ctx, plan.Fields, state.Fields, "fields")
	resp.Diagnostics.Append(fieldsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if fieldsChanged {
		payload["fields"] = fieldsValue
	}
	triggersValue, triggersDiags, triggersChanged := jsonStringPayload(ctx, plan.Triggers, state.Triggers, "triggers")
	resp.Diagnostics.Append(triggersDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if triggersChanged {
		payload["triggers"] = triggersValue
	}
	limitsValue, limitsDiags, limitsChanged := jsonStringPayload(ctx, plan.Limits, state.Limits, "limits")
	resp.Diagnostics.Append(limitsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if limitsChanged {
		payload["limits"] = limitsValue
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
		if err := r.client.Call("update_event", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps event failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *eventResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state eventModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting events.")
		return
	}
	if err := r.client.Call("delete_event", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps event failed", err.Error())
	}
}

func (r *eventResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func eventRequest(ctx context.Context, m eventModel, withID bool) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{"title": m.Title.ValueString(), "enabled": m.Enabled.ValueBool()}
	if withID && !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Category.IsNull() && !m.Category.IsUnknown() {
		payload["category"] = m.Category.ValueString()
	}
	if !m.Plugin.IsNull() && !m.Plugin.IsUnknown() {
		payload["plugin"] = m.Plugin.ValueString()
	}
	var diags diag.Diagnostics
	if !m.Targets.IsNull() && !m.Targets.IsUnknown() {
		var targets []string
		diags.Append(m.Targets.ElementsAs(ctx, &targets, false)...)
		if diags.HasError() {
			return nil, diags
		}
		payload["targets"] = targets
	}
	if !m.Tags.IsNull() && !m.Tags.IsUnknown() {
		var tags []string
		diags.Append(m.Tags.ElementsAs(ctx, &tags, false)...)
		if diags.HasError() {
			return nil, diags
		}
		payload["tags"] = tags
	}
	if !m.Algo.IsNull() && !m.Algo.IsUnknown() {
		payload["algo"] = m.Algo.ValueString()
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() {
		payload["notes"] = m.Notes.ValueString()
	}
	diags.Append(addJSONToPayloadCreate(payload, "params", m.Params, map[string]any{})...)
	diags.Append(addJSONToPayloadCreate(payload, "fields", m.Fields, []any{})...)
	diags.Append(addJSONToPayloadCreate(payload, "triggers", m.Triggers, []any{})...)
	diags.Append(addJSONToPayloadCreate(payload, "limits", m.Limits, []any{})...)
	diags.Append(addJSONToPayloadCreate(payload, "actions", m.Actions, []any{})...)
	return payload, diags
}

func setEventModel(ctx context.Context, model *eventModel, event eventPayload) diag.Diagnostics {
	model.ID = types.StringValue(event.ID)
	model.Title = types.StringValue(event.Title)
	model.Enabled = types.BoolValue(event.Enabled)
	if event.Category != "" || !model.Category.IsNull() {
		model.Category = types.StringValue(event.Category)
	}
	if event.Plugin != "" || !model.Plugin.IsNull() {
		model.Plugin = types.StringValue(event.Plugin)
	}
	if len(event.Targets) > 0 || !model.Targets.IsNull() {
		list, listDiags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(event.Targets))
		if listDiags.HasError() {
			return listDiags
		}
		model.Targets = list
	}
	if len(event.Tags) > 0 || !model.Tags.IsNull() {
		list, listDiags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(event.Tags))
		if listDiags.HasError() {
			return listDiags
		}
		model.Tags = list
	}
	if event.Algo != "" || !model.Algo.IsNull() {
		model.Algo = types.StringValue(event.Algo)
	}
	model.Notes = stringValueOrNull(event.Notes, model.Notes)
	model.Params = jsonStringFromRawOptional(event.Params, model.Params)
	model.Fields = jsonStringFromRawOptional(event.Fields, model.Fields)
	model.Triggers = jsonStringFromRawOptional(event.Triggers, model.Triggers)
	model.Limits = jsonStringFromRawOptional(event.Limits, model.Limits)
	model.Actions = jsonStringFromRawOptional(event.Actions, model.Actions)
	return nil
}
