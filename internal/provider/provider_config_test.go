package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestProviderConfigValue(t *testing.T) {
	tests := []struct {
		name        string
		config      types.String
		environment string
		defaultVal  string
		want        string
	}{
		{
			name:        "configured value takes precedence",
			config:      types.StringValue("configured"),
			environment: "environment",
			defaultVal:  "default",
			want:        "configured",
		},
		{
			name:        "environment is used when configuration is null",
			config:      types.StringNull(),
			environment: "environment",
			defaultVal:  "default",
			want:        "environment",
		},
		{
			name:       "default is used when configuration and environment are absent",
			config:     types.StringNull(),
			defaultVal: "default",
			want:       "default",
		},
		{
			name:        "unknown configuration falls back to environment",
			config:      types.StringUnknown(),
			environment: "environment",
			defaultVal:  "default",
			want:        "environment",
		},
		{
			name:        "explicit empty configuration takes precedence",
			config:      types.StringValue(""),
			environment: "environment",
			defaultVal:  "default",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := providerConfigValue(tt.config, tt.environment, tt.defaultVal); got != tt.want {
				t.Errorf("providerConfigValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProviderConfigure(t *testing.T) {
	t.Run("missing endpoint is diagnosed", func(t *testing.T) {
		t.Setenv("XYOPS_ENDPOINT", "")
		t.Setenv("XYOPS_API_KEY", "")

		resp := configureProviderForTest(t, nil, nil)
		if !resp.Diagnostics.HasError() {
			t.Fatal("Configure() diagnostics have no error, want missing endpoint")
		}
		if got, want := resp.Diagnostics[0].Summary(), "Missing xyOps endpoint"; got != want {
			t.Errorf("diagnostic summary = %q, want %q", got, want)
		}
	})

	t.Run("missing API key is diagnosed", func(t *testing.T) {
		t.Setenv("XYOPS_ENDPOINT", "")
		t.Setenv("XYOPS_API_KEY", "")

		resp := configureProviderForTest(t, stringPointer("http://localhost:5522"), nil)
		if !resp.Diagnostics.HasError() {
			t.Fatal("Configure() diagnostics have no error, want missing API key")
		}
		if got, want := resp.Diagnostics[0].Summary(), "Missing xyOps API key"; got != want {
			t.Errorf("diagnostic summary = %q, want %q", got, want)
		}
	})

	t.Run("invalid endpoint is diagnosed", func(t *testing.T) {
		t.Setenv("XYOPS_ENDPOINT", "")
		t.Setenv("XYOPS_API_KEY", "environment-key")

		resp := configureProviderForTest(t, stringPointer("ftp://xyops.invalid"), nil)
		if !resp.Diagnostics.HasError() {
			t.Fatal("Configure() diagnostics have no error, want invalid endpoint")
		}
		if got, want := resp.Diagnostics[0].Summary(), "Invalid xyOps endpoint"; got != want {
			t.Errorf("diagnostic summary = %q, want %q", got, want)
		}
	})

	t.Run("invalid API key is diagnosed", func(t *testing.T) {
		t.Setenv("XYOPS_ENDPOINT", "")
		t.Setenv("XYOPS_API_KEY", "")

		var calls int
		server := newProviderConfigureTestServerRejectingKey(t, &calls)
		t.Setenv("XYOPS_ENDPOINT", server.URL)

		resp := configureProviderForTest(t, stringPointer(server.URL), stringPointer("wrong-key"))
		if !resp.Diagnostics.HasError() {
			t.Fatal("Configure() diagnostics have no error, want invalid API key")
		}
		if got, want := resp.Diagnostics[0].Summary(), "Invalid xyOps API key"; got != want {
			t.Errorf("diagnostic summary = %q, want %q", got, want)
		}
		if resp.ResourceData != nil || resp.DataSourceData != nil {
			t.Fatal("Configure() returned data, want no client after invalid key")
		}
		if calls != 1 {
			t.Errorf("server calls = %d, want 1", calls)
		}
	})

	t.Run("configured endpoint and key override environment", func(t *testing.T) {
		var configuredCalls, environmentCalls int
		configuredServer := newProviderConfigureTestServer(t, "configured-key", &configuredCalls)
		environmentServer := newProviderConfigureTestServer(t, "environment-key", &environmentCalls)
		t.Setenv("XYOPS_ENDPOINT", environmentServer.URL)
		t.Setenv("XYOPS_API_KEY", "environment-key")

		resp := configureProviderForTest(t, stringPointer(configuredServer.URL), stringPointer("configured-key"))
		if resp.Diagnostics.HasError() {
			t.Fatalf("Configure() diagnostics = %s", resp.Diagnostics)
		}
		if resp.ResourceData == nil {
			t.Fatal("Configure() ResourceData is nil")
		}
		if configuredCalls != 1 || environmentCalls != 0 {
			t.Errorf("server calls: configured = %d, environment = %d; want 1 and 0", configuredCalls, environmentCalls)
		}
	})

	t.Run("environment values are used when configuration is null", func(t *testing.T) {
		var calls int
		server := newProviderConfigureTestServer(t, "environment-key", &calls)
		t.Setenv("XYOPS_ENDPOINT", server.URL)
		t.Setenv("XYOPS_API_KEY", "environment-key")

		resp := configureProviderForTest(t, nil, nil)
		if resp.Diagnostics.HasError() {
			t.Fatalf("Configure() diagnostics = %s", resp.Diagnostics)
		}
		if calls != 1 {
			t.Errorf("server calls = %d, want 1", calls)
		}
	})
}

func configureProviderForTest(t *testing.T, endpoint, apiKey *string) *provider.ConfigureResponse {
	t.Helper()
	ctx := context.Background()
	providerImpl := &xyopsProvider{}
	schemaResp := &provider.SchemaResponse{}
	providerImpl.Schema(ctx, provider.SchemaRequest{}, schemaResp)

	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	values := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		values[name] = tftypes.NewValue(attributeType, nil)
	}
	if endpoint != nil {
		values["endpoint"] = tftypes.NewValue(objectType.AttributeTypes["endpoint"], *endpoint)
	}
	if apiKey != nil {
		values["api_key"] = tftypes.NewValue(objectType.AttributeTypes["api_key"], *apiKey)
	}

	resp := &provider.ConfigureResponse{}
	providerImpl.Configure(ctx, provider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw:    tftypes.NewValue(objectType, values),
			Schema: schemaResp.Schema,
		},
	}, resp)
	return resp
}

func newProviderConfigureTestServerRejectingKey(t *testing.T, calls *int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/app/search_alerts/v1" {
			t.Errorf("request path = %q, want /api/app/search_alerts/v1", r.URL.Path)
		}
		(*calls)++
		_, _ = fmt.Fprint(w, `{"code":"unauthorized","description":"Invalid API key"}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func newProviderConfigureTestServer(t *testing.T, wantAPIKey string, calls *int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/app/search_alerts/v1" {
			t.Errorf("request = %s %s, want GET /api/app/search_alerts/v1", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != wantAPIKey {
			t.Errorf("X-API-Key = %q, want %q", got, wantAPIKey)
		}
		(*calls)++
		_, _ = fmt.Fprint(w, `{"code":0}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func stringPointer(value string) *string { return &value }
