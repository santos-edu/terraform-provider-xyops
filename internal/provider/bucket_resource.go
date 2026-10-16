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
	_ resource.Resource                = (*bucketResource)(nil)
	_ resource.ResourceWithConfigure   = (*bucketResource)(nil)
	_ resource.ResourceWithImportState = (*bucketResource)(nil)
)

type bucketResource struct{ client *xyops.Client }

type bucketModel struct {
	ID      types.String `tfsdk:"id"`
	Title   types.String `tfsdk:"title"`
	Enabled types.Bool   `tfsdk:"enabled"`
	Icon    types.String `tfsdk:"icon"`
	Notes   types.String `tfsdk:"notes"`
	Data    types.String `tfsdk:"data"`
}

type bucketResponse struct {
	Bucket bucketPayload   `json:"bucket"`
	Data   json.RawMessage `json:"data"`
}

type bucketPayload struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Enabled bool   `json:"enabled"`
	Icon    string `json:"icon"`
	Notes   string `json:"notes"`
}

func NewBucketResource() resource.Resource { return &bucketResource{} }

func (r *bucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

func (r *bucketResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":      schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":   schema.StringAttribute{Required: true},
		"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"icon":    schema.StringAttribute{Optional: true},
		"notes":   schema.StringAttribute{Optional: true},
		"data":    schema.StringAttribute{Optional: true, Validators: []validator.String{jsonStringValidator{}}},
	}}
}

func (r *bucketResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *bucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan bucketModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating buckets.")
		return
	}
	payload, diags := bucketRequest(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result bucketResponse
	if err := r.client.Call("create_bucket", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps bucket failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.Bucket.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps bucket failed", "xyOps returned a bucket without an id.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state bucketModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading buckets.")
		return
	}
	var result bucketResponse
	err := r.client.Call("get_bucket", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps bucket failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setBucketModel(&state, result.Bucket, result.Data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *bucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state bucketModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating buckets.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{"id": state.ID.ValueString()}
	addChangedString(payload, "title", plan.Title, state.Title)
	addChangedString(payload, "icon", plan.Icon, state.Icon)
	addChangedString(payload, "notes", plan.Notes, state.Notes)
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		payload["enabled"] = plan.Enabled.ValueBool()
	}
	dataValue, dataDiags, dataChanged := jsonStringPayload(ctx, plan.Data, state.Data, "data")
	resp.Diagnostics.Append(dataDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if dataChanged {
		payload["data"] = dataValue
	}
	if len(payload) > 1 {
		if err := r.client.Call("update_bucket", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps bucket failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state bucketModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting buckets.")
		return
	}
	if err := r.client.Call("delete_bucket", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps bucket failed", err.Error())
	}
}

func (r *bucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func bucketRequest(_ context.Context, m bucketModel, withID bool) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{"title": m.Title.ValueString(), "enabled": m.Enabled.ValueBool()}
	if withID && !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Icon.IsNull() && !m.Icon.IsUnknown() {
		payload["icon"] = m.Icon.ValueString()
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() {
		payload["notes"] = m.Notes.ValueString()
	}
	var diags diag.Diagnostics
	if !m.Data.IsNull() && !m.Data.IsUnknown() {
		var data any
		if err := json.Unmarshal([]byte(m.Data.ValueString()), &data); err != nil {
			diags.AddAttributeError(path.Root("data"), "Invalid JSON", err.Error())
			return nil, diags
		}
		payload["data"] = data
	}
	return payload, diags
}

func setBucketModel(model *bucketModel, bucket bucketPayload, data json.RawMessage) diag.Diagnostics {
	model.ID = types.StringValue(bucket.ID)
	model.Title = types.StringValue(bucket.Title)
	model.Enabled = types.BoolValue(bucket.Enabled)
	model.Icon = stringValueOrNull(bucket.Icon, model.Icon)
	model.Notes = stringValueOrNull(bucket.Notes, model.Notes)
	model.Data = jsonStringFromRawOptional(data, model.Data)
	return nil
}
