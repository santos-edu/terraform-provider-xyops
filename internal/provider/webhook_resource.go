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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*webhookResource)(nil)
	_ resource.ResourceWithConfigure   = (*webhookResource)(nil)
	_ resource.ResourceWithImportState = (*webhookResource)(nil)
)

type webhookResource struct{ client *xyops.Client }

type webhookModel struct {
	ID            types.String `tfsdk:"id"`
	Title         types.String `tfsdk:"title"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	Icon          types.String `tfsdk:"icon"`
	URL           types.String `tfsdk:"url"`
	Method        types.String `tfsdk:"method"`
	Headers       types.String `tfsdk:"headers"`
	Body          types.String `tfsdk:"body"`
	Timeout       types.Int64  `tfsdk:"timeout"`
	Retries       types.Int64  `tfsdk:"retries"`
	Follow        types.Bool   `tfsdk:"follow"`
	SSLCertBypass types.Bool   `tfsdk:"ssl_cert_bypass"`
	MaxPerDay     types.Int64  `tfsdk:"max_per_day"`
	Notes         types.String `tfsdk:"notes"`
}

type webhookResponse struct {
	WebHook webhookPayload `json:"web_hook"`
}

type webhookPayload struct {
	ID            string          `json:"id"`
	Title         string          `json:"title"`
	Enabled       bool            `json:"enabled"`
	Icon          string          `json:"icon"`
	URL           string          `json:"url"`
	Method        string          `json:"method"`
	Headers       json.RawMessage `json:"headers"`
	Body          string          `json:"body"`
	Timeout       int64           `json:"timeout"`
	Retries       int64           `json:"retries"`
	Follow        bool            `json:"follow"`
	SSLCertBypass bool            `json:"ssl_cert_bypass"`
	MaxPerDay     int64           `json:"max_per_day"`
	Notes         string          `json:"notes"`
}

func NewWebhookResource() resource.Resource { return &webhookResource{} }

func (r *webhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *webhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":              schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"title":           schema.StringAttribute{Required: true},
		"enabled":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"icon":            schema.StringAttribute{Optional: true},
		"url":             schema.StringAttribute{Required: true, Sensitive: true},
		"method":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("POST"), Validators: []validator.String{enumValidator{allowed: []string{"GET", "HEAD", "POST", "PUT", "DELETE"}}}},
		"headers":         schema.StringAttribute{Optional: true, Sensitive: true, Validators: []validator.String{jsonStringValidator{}}},
		"body":            schema.StringAttribute{Optional: true, Sensitive: true},
		"timeout":         schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(30)},
		"retries":         schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0)},
		"follow":          schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		"ssl_cert_bypass": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		"max_per_day":     schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0)},
		"notes":           schema.StringAttribute{Optional: true},
	}}
}

func (r *webhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *webhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before creating webhooks.")
		return
	}
	payload, diags := webhookRequest(ctx, plan, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result webhookResponse
	if err := r.client.Call("create_web_hook", payload, &result); err != nil {
		resp.Diagnostics.AddError("Create xyOps webhook failed", err.Error())
		return
	}
	plan.ID = types.StringValue(result.WebHook.ID)
	if plan.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Create xyOps webhook failed", "xyOps returned a webhook without an id.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before reading webhooks.")
		return
	}
	var result webhookResponse
	err := r.client.Call("get_web_hook", map[string]any{"id": state.ID.ValueString()}, &result)
	if err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read xyOps webhook failed", err.Error())
		return
	}
	resp.Diagnostics.Append(setWebhookModel(ctx, &state, result.WebHook)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *webhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before updating webhooks.")
		return
	}
	resolveUnknowns(&plan, &state)
	payload := map[string]any{"id": state.ID.ValueString()}
	addChangedString(payload, "title", plan.Title, state.Title)
	addChangedString(payload, "url", plan.URL, state.URL)
	addChangedString(payload, "method", plan.Method, state.Method)
	addChangedString(payload, "body", plan.Body, state.Body)
	addChangedString(payload, "icon", plan.Icon, state.Icon)
	addChangedString(payload, "notes", plan.Notes, state.Notes)
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		payload["enabled"] = plan.Enabled.ValueBool()
	}
	if plan.Timeout.ValueInt64() != state.Timeout.ValueInt64() {
		payload["timeout"] = plan.Timeout.ValueInt64()
	}
	if plan.Retries.ValueInt64() != state.Retries.ValueInt64() {
		payload["retries"] = plan.Retries.ValueInt64()
	}
	if plan.Follow.ValueBool() != state.Follow.ValueBool() {
		payload["follow"] = plan.Follow.ValueBool()
	}
	if plan.SSLCertBypass.ValueBool() != state.SSLCertBypass.ValueBool() {
		payload["ssl_cert_bypass"] = plan.SSLCertBypass.ValueBool()
	}
	if plan.MaxPerDay.ValueInt64() != state.MaxPerDay.ValueInt64() {
		payload["max_per_day"] = plan.MaxPerDay.ValueInt64()
	}
	headers, hdiags, hchanged := jsonStringPayload(ctx, plan.Headers, state.Headers, "headers")
	resp.Diagnostics.Append(hdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if hchanged {
		payload["headers"] = headers
	}
	if len(payload) > 1 {
		if err := r.client.Call("update_web_hook", payload, nil); err != nil {
			resp.Diagnostics.AddError("Update xyOps webhook failed", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("xyOps client unavailable", "Configure the xyOps provider before deleting webhooks.")
		return
	}
	if err := r.client.Call("delete_web_hook", map[string]string{"id": state.ID.ValueString()}, nil); err != nil {
		var apiErr *xyops.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Delete xyOps webhook failed", err.Error())
	}
}

func (r *webhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func webhookRequest(ctx context.Context, m webhookModel, withID bool) (map[string]any, diag.Diagnostics) {
	payload := map[string]any{
		"title":           m.Title.ValueString(),
		"enabled":         m.Enabled.ValueBool(),
		"url":             m.URL.ValueString(),
		"method":          m.Method.ValueString(),
		"timeout":         m.Timeout.ValueInt64(),
		"retries":         m.Retries.ValueInt64(),
		"follow":          m.Follow.ValueBool(),
		"ssl_cert_bypass": m.SSLCertBypass.ValueBool(),
		"max_per_day":     m.MaxPerDay.ValueInt64(),
	}
	if withID && !m.ID.IsNull() && !m.ID.IsUnknown() {
		payload["id"] = m.ID.ValueString()
	}
	if !m.Icon.IsNull() && !m.Icon.IsUnknown() {
		payload["icon"] = m.Icon.ValueString()
	}
	if !m.Body.IsNull() && !m.Body.IsUnknown() {
		payload["body"] = m.Body.ValueString()
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() {
		payload["notes"] = m.Notes.ValueString()
	}
	var diags diag.Diagnostics
	if !m.Headers.IsNull() && !m.Headers.IsUnknown() {
		var headers any
		if err := json.Unmarshal([]byte(m.Headers.ValueString()), &headers); err != nil {
			diags.AddAttributeError(path.Root("headers"), "Invalid JSON", err.Error())
			return nil, diags
		}
		payload["headers"] = headers
	}
	return payload, diags
}

func setWebhookModel(_ context.Context, model *webhookModel, hook webhookPayload) diag.Diagnostics {
	model.ID = types.StringValue(hook.ID)
	model.Title = types.StringValue(hook.Title)
	model.Enabled = types.BoolValue(hook.Enabled)
	model.URL = types.StringValue(hook.URL)
	model.Method = stringValueOrNull(hook.Method, model.Method)
	model.Timeout = types.Int64Value(hook.Timeout)
	model.Retries = types.Int64Value(hook.Retries)
	model.Follow = types.BoolValue(hook.Follow)
	model.SSLCertBypass = types.BoolValue(hook.SSLCertBypass)
	model.MaxPerDay = types.Int64Value(hook.MaxPerDay)
	model.Icon = stringValueOrNull(hook.Icon, model.Icon)
	model.Body = stringValueOrNull(hook.Body, model.Body)
	model.Notes = stringValueOrNull(hook.Notes, model.Notes)
	model.Headers = jsonStringFromRawOptional(hook.Headers, model.Headers)
	return nil
}
