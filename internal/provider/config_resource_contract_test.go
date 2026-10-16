package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"sync/atomic"
	"testing"

	"github.com/santos-edu/terraform-provider-xyops/internal/xyops"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestConfigResourceImportIsUnsupported(t *testing.T) {
	resourceImpl := NewConfigResource().(resource.ResourceWithImportState)
	ctx := context.Background()
	resp := &resource.ImportStateResponse{}
	resourceImpl.ImportState(ctx, resource.ImportStateRequest{ID: configSyntheticID}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("ImportState() has no diagnostics, want import unsupported error")
	}
	if got, want := resp.Diagnostics[0].Summary(), "Import not supported"; got != want {
		t.Errorf("diagnostic summary = %q, want %q", got, want)
	}
}

func TestConfigResourceDeleteClearsActions(t *testing.T) {
	ctx := context.Background()
	resourceImpl := NewConfigResource()
	schemaResp := &resource.SchemaResponse{}
	resourceImpl.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema() diagnostics = %s", schemaResp.Diagnostics)
	}
	state := resourceStateWithID(t, schemaResp.Schema, configSyntheticID)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodPost; got != want {
			t.Errorf("request method = %q, want %q", got, want)
		}
		if got, want := r.URL.Path, "/api/app/admin_update_config/v1"; got != want {
			t.Errorf("request path = %q, want %q", got, want)
		}
		if got := r.Header.Get("X-API-Key"); got != "test-key" {
			t.Errorf("X-API-Key = %q, want %q", got, "test-key")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %s", err)
		}
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("decode request body: %s", err)
		}
		want := map[string]any{"job_universal_actions": map[string]any{"default": []any{}, "workflow": []any{}}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("request body = %#v, want %#v", got, want)
		}
		_, _ = io.WriteString(w, `{"code":0}`)
	}))
	defer server.Close()

	client, err := xyops.NewClient(server.URL, "test-key")
	if err != nil {
		t.Fatalf("NewClient() error = %s", err)
	}
	configured, ok := resourceImpl.(resource.ResourceWithConfigure)
	if !ok {
		t.Fatalf("resource %T does not implement Configure", resourceImpl)
	}
	configureResp := &resource.ConfigureResponse{}
	configured.Configure(ctx, resource.ConfigureRequest{ProviderData: client}, configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatalf("Configure() diagnostics = %s", configureResp.Diagnostics)
	}

	deleter, ok := resourceImpl.(interface {
		Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse)
	})
	if !ok {
		t.Fatalf("resource %T does not implement Delete", resourceImpl)
	}
	deleteResp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	deleter.Delete(ctx, resource.DeleteRequest{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}, deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("Delete() diagnostics = %s", deleteResp.Diagnostics)
	}
}

func TestConfigResourcePreventDestroy(t *testing.T) {
	var resetCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/app/search_alerts/v1":
			_, _ = io.WriteString(w, `{"code":0}`)
		case "/api/app/admin_get_config/v1":
			_, _ = io.WriteString(w, `{"code":0,"config":{"job_universal_actions":{"default":[],"workflow":[]}}}`)
		case "/api/app/admin_update_config/v1":
			resetCalls.Add(1)
			_, _ = io.WriteString(w, `{"code":0}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	configWithProtection := providerConfig() + `
resource "xyops_config" "test" {
  job_universal_actions = jsonencode({ default = [], workflow = [] })
  lifecycle { prevent_destroy = true }
}
`
	configWithoutProtection := providerConfig() + `
resource "xyops_config" "test" {
  job_universal_actions = jsonencode({ default = [], workflow = [] })
}
`
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{Config: configWithProtection},
			{
				Config:      configWithProtection,
				Destroy:     true,
				ExpectError: regexp.MustCompile(`(?i)prevent_destroy`),
			},
			{
				Config: configWithoutProtection,
				ConfigPlanChecks: tfresource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{Config: providerConfig()},
		},
	})
	if got, want := resetCalls.Load(), int32(2); got != want {
		t.Errorf("admin_update_config calls = %d, want %d (create and final destroy only)", got, want)
	}
}
