package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestJSONStringValidator(t *testing.T) {
	tests := []struct {
		name      string
		value     types.String
		wantError bool
	}{
		{name: "object JSON", value: types.StringValue(`{"key":"value"}`)},
		{name: "array JSON", value: types.StringValue(`[1,true,null]`)},
		{name: "null Terraform value", value: types.StringNull()},
		{name: "unknown Terraform value", value: types.StringUnknown()},
		{name: "invalid JSON", value: types.StringValue(`{"key":`), wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := validateString(t, jsonStringValidator{}, tt.value)
			assertValidationError(t, resp, tt.wantError, "Invalid JSON", "The value must contain valid JSON.")
		})
	}
}

func TestEnumValidator(t *testing.T) {
	tests := []struct {
		name      string
		value     types.String
		wantError bool
	}{
		{name: "first allowed value", value: types.StringValue("event")},
		{name: "second allowed value", value: types.StringValue("monitor")},
		{name: "null Terraform value", value: types.StringNull()},
		{name: "unknown Terraform value", value: types.StringUnknown()},
		{name: "unsupported value", value: types.StringValue("workflow"), wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := validateString(t, enumValidator{allowed: []string{"event", "monitor"}}, tt.value)
			assertValidationError(t, resp, tt.wantError, "Invalid value", "Value must be one of: event, monitor.")
		})
	}
}

func TestServerIDRegexValidator(t *testing.T) {
	message := "The value must be a valid xyOps server id: lowercase letters, digits and underscores only."
	validator := newRegexStringValidator(`^[a-z0-9_]+$`, message)
	tests := []struct {
		name      string
		value     types.String
		wantError bool
	}{
		{name: "lowercase id", value: types.StringValue("server_01")},
		{name: "null Terraform value", value: types.StringNull()},
		{name: "unknown Terraform value", value: types.StringUnknown()},
		{name: "uppercase id", value: types.StringValue("Server_01"), wantError: true},
		{name: "hyphenated id", value: types.StringValue("server-01"), wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := validateString(t, validator, tt.value)
			assertValidationError(t, resp, tt.wantError, "Invalid value", message)
		})
	}
}

func validateString(t *testing.T, stringValidator validator.String, value types.String) *validator.StringResponse {
	t.Helper()
	resp := &validator.StringResponse{}
	stringValidator.ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("value"),
		ConfigValue: value,
	}, resp)
	return resp
}

func assertValidationError(t *testing.T, resp *validator.StringResponse, wantError bool, summary, detail string) {
	t.Helper()
	if got := resp.Diagnostics.HasError(); got != wantError {
		t.Fatalf("HasError() = %t, want %t: %s", got, wantError, resp.Diagnostics)
	}
	if !wantError {
		return
	}
	if got := resp.Diagnostics[0].Summary(); got != summary {
		t.Errorf("diagnostic summary = %q, want %q", got, summary)
	}
	if got := resp.Diagnostics[0].Detail(); got != detail {
		t.Errorf("diagnostic detail = %q, want %q", got, detail)
	}
}
