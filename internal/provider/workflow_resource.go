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
	_ resource.Resource                = (*workflowResource)(nil)
	_ resource.ResourceWithConfigure   = (*workflowResource)(nil)
	_ resource.ResourceWithImportState = (*workflowResource)(nil)
)

type workflowResource struct{ client *xyops.Client }

type workflowModel struct {
	ID       types.String `tfsdk:"id"`
	Title    types.String `tfsdk:"title"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	Category types.String `tfsdk:"category"`
	Tags     types.List   `tfsdk:"tags"`
	Triggers types.String `tfsdk:"triggers"`
	Actions  types.String `tfsdk:"actions"`
	Limits   types.String `tfsdk:"limits"`
	Workflow types.String `tfsdk:"workflow"`
}

func NewWorkflowResource() resource.Resource { return &workflowResource{} }

func (r *workflowResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflow"
}

func (r *workflowResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":       schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":    schema.StringAttribute{Required: true},
		"enabled":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"category": schema.StringAttribute{Optional: true},
		"tags":     schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType},
		"triggers": schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
		"actions":  schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
		"limits":   schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
		"workflow": schema.StringAttribute{Required: true, Validators: []validator.String{jsonStringValidator{}}},
	}}
}

func (r *workflowResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *workflowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workflowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating workflows.")
		return
	}
	payload, diags := workflowRequest(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result eventResponse
	if err := r.client.Call("create_event", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps workflow failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.Event.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps workflow failed", "xyOps returned a workflow without an id.")
		return
	}
	// setWorkflowModel cannot be reused here: create_event always returns
	// triggers, actions and limits as [], which would overwrite the null the plan
	// holds for those non-computed attributes.
	tags, tagsDiags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(result.Event.Tags))
	if tagsDiags.HasError() {
		resp.Diagnostics.Append(tagsDiags...)
		return
	}
	plan.Tags = tags
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workflowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workflowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading workflows.")
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
		resp.Diagnostics.AddError("Read xyOps workflow failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setWorkflowModel(ctx, &state, result.Event)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *workflowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state workflowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating workflows.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{
		"id":      state.ID.ValueString(),
		"plugin":  "_workflow",
		"type":    "workflow",
		"targets": []string{},
		"fields":  []any{},
	}
	hasChanges := false
	addChangedString(payload, "title", plan.Title, state.Title)
	if _, ok := payload["title"]; ok {
		hasChanges = true
	}
	addChangedString(payload, "category", plan.Category, state.Category)
	if _, ok := payload["category"]; ok {
		hasChanges = true
	}
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		payload["enabled"] = plan.Enabled.ValueBool()
		hasChanges = true
	}
	tags, tdiags := changedGroups(ctx, plan.Tags, state.Tags)
	resp.Diagnostics.Append(tdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if tags != nil {
		payload["tags"] = tags
		hasChanges = true
	}
	triggersValue, triggersDiags, triggersChanged := jsonStringPayload(ctx, plan.Triggers, state.Triggers, "triggers")
	resp.Diagnostics.Append(triggersDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if triggersChanged {
		payload["triggers"] = triggersValue
		hasChanges = true
	}
	actionsValue, actionsDiags, actionsChanged := jsonStringPayload(ctx, plan.Actions, state.Actions, "actions")
	resp.Diagnostics.Append(actionsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if actionsChanged {
		payload["actions"] = actionsValue
		hasChanges = true
	}
	limitsValue, limitsDiags, limitsChanged := jsonStringPayload(ctx, plan.Limits, state.Limits, "limits")
	resp.Diagnostics.Append(limitsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if limitsChanged {
		payload["limits"] = limitsValue
		hasChanges = true
	}
	workflowValue, workflowDiags, workflowChanged := jsonStringPayload(ctx, plan.Workflow, state.Workflow, "workflow")
	resp.Diagnostics.Append(workflowDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if workflowChanged {
		payload["workflow"] = workflowValue
		hasChanges = true
	}
	if hasChanges {
		if err := r.client.Call("update_event", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps workflow failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workflowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workflowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting workflows.")
		return
	}
	if err := r.client.Call("delete_event", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps workflow failed", err.Error())
	}
}

func (r *workflowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func workflowRequest(ctx context.Context, m workflowModel, withID bool) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{
		"title":   m.Title.ValueString(),
		"enabled": m.Enabled.ValueBool(),
		"plugin":  "_workflow",
		"type":    "workflow",
		"targets": []string{},
		"fields":  []any{},
	}
	if withID && !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Category.IsNull() && !m.Category.IsUnknown() {
		payload["category"] = m.Category.ValueString()
	}
	var diags diag.Diagnostics
	if !m.Tags.IsNull() && !m.Tags.IsUnknown() {
		var tags []string
		diags.Append(m.Tags.ElementsAs(ctx, &tags, false)...)
		if diags.HasError() {
			return nil, diags
		}
		payload["tags"] = tags
	}
	diags.Append(addJSONToPayloadCreate(payload, "triggers", m.Triggers, []any{})...)
	diags.Append(addJSONToPayloadCreate(payload, "actions", m.Actions, []any{})...)
	diags.Append(addJSONToPayloadCreate(payload, "limits", m.Limits, []any{})...)
	if m.Workflow.IsNull() || m.Workflow.IsUnknown() {
		diags.AddAttributeError(path.Root("workflow"), "Workflow is required", "")
		return nil, diags
	}
	var workflowObj map[string]any
	if err := json.Unmarshal([]byte(m.Workflow.ValueString()), &workflowObj); err != nil {
		diags.AddAttributeError(path.Root("workflow"), "Invalid JSON", err.Error())
		return nil, diags
	}
	nodes, ok := workflowObj["nodes"].([]any)
	if ok {
		for i, n := range nodes {
			node, ok := n.(map[string]any)
			if !ok {
				diags.AddAttributeError(path.Root("workflow").AtMapKey("nodes").AtTupleIndex(i), "Invalid node", "workflow node must be an object")
				continue
			}
			if _, hasX := node["x"]; !hasX {
				diags.AddAttributeError(path.Root("workflow").AtMapKey("nodes").AtTupleIndex(i), "Missing x", "workflow node must have x coordinate")
			}
			if _, hasY := node["y"]; !hasY {
				diags.AddAttributeError(path.Root("workflow").AtMapKey("nodes").AtTupleIndex(i), "Missing y", "workflow node must have y coordinate")
			}
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	payload["workflow"] = workflowObj
	return payload, diags
}

func setWorkflowModel(ctx context.Context, model *workflowModel, event eventPayload) diag.Diagnostics {
	model.ID = types.StringValue(event.ID)
	model.Title = types.StringValue(event.Title)
	model.Enabled = types.BoolValue(event.Enabled)
	if event.Category != "" || !model.Category.IsNull() {
		model.Category = types.StringValue(event.Category)
	}
	list, listDiags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(event.Tags))
	if listDiags.HasError() {
		return listDiags
	}
	model.Tags = list
	model.Triggers = jsonStringFromRawOptional(event.Triggers, model.Triggers)
	model.Actions = jsonStringFromRawOptional(event.Actions, model.Actions)
	model.Limits = jsonStringFromRawOptional(event.Limits, model.Limits)
	model.Workflow = jsonStringFromRaw(event.Workflow, model.Workflow)
	return nil
}
