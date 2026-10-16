package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/santos-edu/terraform-provider-xyops/internal/xyops"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceSchema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type resourceReader interface {
	resource.ResourceWithConfigure
	Read(context.Context, resource.ReadRequest, *resource.ReadResponse)
}

type resourceDeleter interface {
	resource.ResourceWithConfigure
	Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse)
}

func TestResourceReadErrors(t *testing.T) {
	tests := []struct {
		name   string
		new    func() resource.Resource
		action string
		method string
	}{
		{name: "alert", new: NewAlertResource, action: "get_alert", method: http.MethodPost},
		{name: "bucket", new: NewBucketResource, action: "get_bucket", method: http.MethodPost},
		{name: "tag", new: NewTagResource, action: "get_tag", method: http.MethodPost},
		{name: "group", new: NewGroupResource, action: "get_group", method: http.MethodPost},
		{name: "category", new: NewCategoryResource, action: "get_category", method: http.MethodPost},
		{name: "plugin", new: NewPluginResource, action: "get_plugin", method: http.MethodPost},
		{name: "webhook", new: NewWebhookResource, action: "get_web_hook", method: http.MethodPost},
		{name: "event", new: NewEventResource, action: "get_event", method: http.MethodPost},
		{name: "workflow", new: NewWorkflowResource, action: "get_event", method: http.MethodPost},
		{name: "monitor", new: NewMonitorResource, action: "get_monitor", method: http.MethodPost},
		{name: "config", new: NewConfigResource, action: "admin_get_config", method: http.MethodGet},
		{name: "role", new: NewRoleResource, action: "get_role", method: http.MethodPost},
		{name: "api_key", new: NewAPIKeyResource, action: "get_api_key", method: http.MethodPost},
		{name: "channel", new: NewChannelResource, action: "get_channel", method: http.MethodPost},
		{name: "secret", new: NewSecretResource, action: "get_secret", method: http.MethodPost},
		{name: "server", new: NewServerResource, action: "get_server", method: http.MethodPost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, response := range []struct {
				name        string
				body        string
				wantRemoved bool
			}{
				{name: "notfound removes resource", body: `{"code":"notfound","description":"missing"}`, wantRemoved: true},
				{name: "other API error is diagnostic", body: `{"code":"api","description":"invalid request"}`},
			} {
				t.Run(response.name, func(t *testing.T) {
					resourceImpl := tt.new()
					reader, ok := resourceImpl.(resourceReader)
					if !ok {
						t.Fatalf("resource %T does not implement Read and Configure", resourceImpl)
					}

					ctx := context.Background()
					schemaResp := &resource.SchemaResponse{}
					resourceImpl.Schema(ctx, resource.SchemaRequest{}, schemaResp)
					if schemaResp.Diagnostics.HasError() {
						t.Fatalf("Schema() diagnostics = %s", schemaResp.Diagnostics)
					}
					state := resourceStateWithID(t, schemaResp.Schema, "test-id")

					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if got, want := r.Method, tt.method; got != want {
							t.Errorf("request method = %q, want %q", got, want)
						}
						if got, want := r.URL.Path, "/api/app/"+tt.action+"/v1"; got != want {
							t.Errorf("request path = %q, want %q", got, want)
						}
						if got := r.Header.Get("X-API-Key"); got != "test-key" {
							t.Errorf("X-API-Key = %q, want %q", got, "test-key")
						}
						if tt.method == http.MethodPost {
							body, err := io.ReadAll(r.Body)
							if err != nil {
								t.Errorf("read request body: %s", err)
							}
							if got, want := string(body), `{"id":"test-id"}`; got != want {
								t.Errorf("request body = %s, want %s", got, want)
							}
						}
						_, _ = fmt.Fprint(w, response.body)
					}))
					t.Cleanup(server.Close)

					client, err := xyops.NewClient(server.URL, "test-key")
					if err != nil {
						t.Fatalf("NewClient() error = %s", err)
					}
					configureResp := &resource.ConfigureResponse{}
					reader.Configure(ctx, resource.ConfigureRequest{ProviderData: client}, configureResp)
					if configureResp.Diagnostics.HasError() {
						t.Fatalf("Configure() diagnostics = %s", configureResp.Diagnostics)
					}

					readResp := &resource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
					reader.Read(ctx, resource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}, readResp)

					if response.wantRemoved {
						if readResp.Diagnostics.HasError() {
							t.Fatalf("Read() diagnostics = %s, want none for notfound", readResp.Diagnostics)
						}
						if !readResp.State.Raw.IsNull() {
							t.Errorf("Read() state = %s, want resource removed", readResp.State.Raw)
						}
						return
					}
					if !readResp.Diagnostics.HasError() {
						t.Fatal("Read() has no diagnostics, want non-notfound API error")
					}
					if !readResp.State.Raw.Equal(state) {
						t.Errorf("Read() changed state on API error: got %s, want %s", readResp.State.Raw, state)
					}
				})
			}
		})
	}
}

func TestResourceDeleteNotFoundIsIdempotent(t *testing.T) {
	tests := []struct {
		name   string
		new    func() resource.Resource
		action string
	}{
		{name: "alert", new: NewAlertResource, action: "delete_alert"},
		{name: "bucket", new: NewBucketResource, action: "delete_bucket"},
		{name: "tag", new: NewTagResource, action: "delete_tag"},
		{name: "group", new: NewGroupResource, action: "delete_group"},
		{name: "category", new: NewCategoryResource, action: "delete_category"},
		{name: "plugin", new: NewPluginResource, action: "delete_plugin"},
		{name: "webhook", new: NewWebhookResource, action: "delete_web_hook"},
		{name: "event", new: NewEventResource, action: "delete_event"},
		{name: "workflow", new: NewWorkflowResource, action: "delete_event"},
		{name: "monitor", new: NewMonitorResource, action: "delete_monitor"},
		{name: "role", new: NewRoleResource, action: "delete_role"},
		{name: "api_key", new: NewAPIKeyResource, action: "delete_api_key"},
		{name: "channel", new: NewChannelResource, action: "delete_channel"},
		{name: "secret", new: NewSecretResource, action: "delete_secret"},
		{name: "server", new: NewServerResource, action: "delete_server"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceImpl := tt.new()
			deleter, ok := resourceImpl.(resourceDeleter)
			if !ok {
				t.Fatalf("resource %T does not implement Delete and Configure", resourceImpl)
			}

			ctx := context.Background()
			schemaResp := &resource.SchemaResponse{}
			resourceImpl.Schema(ctx, resource.SchemaRequest{}, schemaResp)
			if schemaResp.Diagnostics.HasError() {
				t.Fatalf("Schema() diagnostics = %s", schemaResp.Diagnostics)
			}
			state := resourceStateWithID(t, schemaResp.Schema, "test-id")

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got, want := r.Method, http.MethodPost; got != want {
					t.Errorf("request method = %q, want %q", got, want)
				}
				if got, want := r.URL.Path, "/api/app/"+tt.action+"/v1"; got != want {
					t.Errorf("request path = %q, want %q", got, want)
				}
				if got := r.Header.Get("X-API-Key"); got != "test-key" {
					t.Errorf("X-API-Key = %q, want %q", got, "test-key")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read request body: %s", err)
				}
				if got, want := string(body), `{"id":"test-id"}`; got != want {
					t.Errorf("request body = %s, want %s", got, want)
				}
				_, _ = fmt.Fprint(w, `{"code":"notfound","description":"already deleted"}`)
			}))
			t.Cleanup(server.Close)

			client, err := xyops.NewClient(server.URL, "test-key")
			if err != nil {
				t.Fatalf("NewClient() error = %s", err)
			}
			configureResp := &resource.ConfigureResponse{}
			deleter.Configure(ctx, resource.ConfigureRequest{ProviderData: client}, configureResp)
			if configureResp.Diagnostics.HasError() {
				t.Fatalf("Configure() diagnostics = %s", configureResp.Diagnostics)
			}

			deleteResp := &resource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
			deleter.Delete(ctx, resource.DeleteRequest{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}, deleteResp)
			if deleteResp.Diagnostics.HasError() {
				t.Errorf("Delete() diagnostics = %s, want none when API returns notfound", deleteResp.Diagnostics)
			}
		})
	}
}

func resourceStateWithID(t *testing.T, schema resourceSchema.Schema, id string) tftypes.Value {
	t.Helper()
	ctx := context.Background()
	objectType, ok := schema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("resource schema type = %T, want tftypes.Object", schema.Type().TerraformType(ctx))
	}
	values := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		values[name] = tftypes.NewValue(attributeType, nil)
	}
	idType, ok := objectType.AttributeTypes["id"]
	if !ok {
		t.Fatal("resource schema does not contain an id attribute")
	}
	values["id"] = tftypes.NewValue(idType, id)
	return tftypes.NewValue(objectType, values)
}
