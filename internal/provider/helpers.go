package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func addChangedString(payload map[string]any, name string, plan, state types.String) {
	if plan.IsUnknown() {
		return
	}
	if plan.IsNull() && !state.IsNull() {
		payload[name] = ""
	} else if !plan.IsNull() && (state.IsNull() || plan.ValueString() != state.ValueString()) {
		payload[name] = plan.ValueString()
	}
}

func changedGroups(ctx context.Context, plan, state types.List) ([]string, diag.Diagnostics) {
	if plan.IsUnknown() {
		return nil, nil
	}
	var planValues, stateValues []string
	var diags diag.Diagnostics
	if !plan.IsNull() {
		diags.Append(plan.ElementsAs(ctx, &planValues, false)...)
		if diags.HasError() {
			return nil, diags
		}
	}
	if !state.IsNull() && !state.IsUnknown() {
		diags.Append(state.ElementsAs(ctx, &stateValues, false)...)
		if diags.HasError() {
			return nil, diags
		}
	}
	if slices.Equal(planValues, stateValues) {
		return nil, nil
	}
	return planValues, nil
}

func normalizeJSONString(s string) (string, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

func jsonStringFromRaw(raw json.RawMessage, state types.String) types.String {
	s := string(raw)
	if (s == "" || s == "null") && state.IsNull() {
		return types.StringNull()
	}
	canonical, err := normalizeJSONString(s)
	if err != nil {
		if !state.IsNull() {
			return state
		}
		return types.StringNull()
	}
	return types.StringValue(canonical)
}

// jsonStringFromRawOptional keeps null when the configuration omitted the
// attribute, instead of adopting the empty container the API echoes back, so an
// Optional (non-Computed) JSON field does not drift forever.
func jsonStringFromRawOptional(raw json.RawMessage, state types.String) types.String {
	if state.IsNull() {
		return types.StringNull()
	}
	return jsonStringFromRaw(raw, state)
}

// resolveUnknowns replaces every planned attribute that is still unknown with
// the prior state. An attribute is unknown when the configuration did not set
// it and the framework could not default it, or when a reference Terraform has
// not resolved; either way the provider never sends it to the API, so the
// previous read is the value to keep. Writing it back also stops Terraform from
// aborting the apply with "not wholly known".
func resolveUnknowns(plan, state any) {
	pv := reflect.ValueOf(plan).Elem()
	sv := reflect.ValueOf(state).Elem()
	for i := 0; i < pv.NumField(); i++ {
		field := pv.Field(i)
		if !field.CanSet() {
			continue
		}
		value, ok := field.Interface().(interface{ IsUnknown() bool })
		if !ok || !value.IsUnknown() {
			continue
		}
		field.Set(sv.Field(i))
	}
}

func stringValueOrNull(value string, state types.String) types.String {
	if value == "" && state.IsNull() {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func int64ValueOrNull(value int64, state types.Int64) types.Int64 {
	if value == 0 && state.IsNull() {
		return types.Int64Null()
	}
	return types.Int64Value(value)
}

func jsonStringPayload(ctx context.Context, plan, state types.String, attrName string) (any, diag.Diagnostics, bool) {
	var diags diag.Diagnostics
	if plan.IsUnknown() {
		return nil, diags, false
	}
	if plan.IsNull() {
		if state.IsNull() {
			return nil, diags, false
		}
		return emptyContainerFromState(state), diags, true
	}
	var planValue any
	if err := json.Unmarshal([]byte(plan.ValueString()), &planValue); err != nil {
		diags.AddAttributeError(path.Root(attrName), "Invalid JSON", err.Error())
		return nil, diags, false
	}
	if !state.IsNull() && !state.IsUnknown() {
		var stateValue any
		if err := json.Unmarshal([]byte(state.ValueString()), &stateValue); err != nil {
			diags.AddAttributeError(path.Root(attrName), "Invalid JSON in state", err.Error())
			return nil, diags, false
		}
		if reflect.DeepEqual(planValue, stateValue) {
			return nil, diags, false
		}
	}
	return planValue, diags, true
}

func emptyContainerFromState(state types.String) any {
	if !state.IsNull() && !state.IsUnknown() {
		s := state.ValueString()
		if s == "[]" || (len(s) > 0 && s[0] == '[') {
			return []any{}
		}
	}
	return map[string]any{}
}

func addJSONToPayloadCreate(payload map[string]any, name string, value types.String, empty any) diag.Diagnostics {
	if value.IsNull() || value.IsUnknown() {
		payload[name] = empty
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(value.ValueString()), &v); err != nil {
		var diags diag.Diagnostics
		diags.AddAttributeError(path.Root(name), "Invalid JSON", err.Error())
		return diags
	}
	payload[name] = v
	return nil
}

type enumValidator struct{ allowed []string }

func (v enumValidator) Description(context.Context) string {
	return fmt.Sprintf("Value must be one of: %s.", strings.Join(v.allowed, ", "))
}

func (v enumValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v enumValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if slices.Contains(v.allowed, req.ConfigValue.ValueString()) {
		return
	}
	resp.Diagnostics.AddAttributeError(req.Path, "Invalid value", v.Description(ctx))
}

type regexStringValidator struct {
	pattern *regexp.Regexp
	message string
}

func newRegexStringValidator(pattern, message string) regexStringValidator {
	return regexStringValidator{pattern: regexp.MustCompile(pattern), message: message}
}

func (v regexStringValidator) Description(context.Context) string { return v.message }

func (v regexStringValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v regexStringValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if !v.pattern.MatchString(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid value", v.Description(ctx))
	}
}

type jsonStringValidator struct{}

func (jsonStringValidator) Description(context.Context) string { return "Value must be valid JSON." }

func (jsonStringValidator) MarkdownDescription(ctx context.Context) string {
	return jsonStringValidator{}.Description(ctx)
}

func (jsonStringValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if !json.Valid([]byte(req.ConfigValue.ValueString())) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid JSON", "The value must contain valid JSON.")
	}
}
