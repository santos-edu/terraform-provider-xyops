package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/santos-edu/terraform-provider-xyops/internal/xyops"
)

// A config that points a managed attribute at a value Terraform cannot know at
// plan time delivers that attribute unknown to Update. The same happens to an
// omitted Optional+Computed attribute without a default. Update has to keep the
// previous state for every unknown it does not write, otherwise it either sends
// the zero value to the API or aborts the apply with "not wholly known".
// Terraform resolves references between resources before calling the provider,
// so this path is only reachable by driving Update directly.
func TestResourceUpdate_unknownPlanValues(t *testing.T) {
	ctx := context.Background()
	str := func(v string) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }
	num := func(v int64) tftypes.Value { return tftypes.NewValue(tftypes.Number, v) }
	boolean := func(v bool) tftypes.Value { return tftypes.NewValue(tftypes.Bool, v) }
	strList := func(v ...string) tftypes.Value {
		values := make([]tftypes.Value, len(v))
		for i, s := range v {
			values[i] = str(s)
		}
		return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, values)
	}
	numMap := func(v map[string]int64) tftypes.Value {
		values := make(map[string]tftypes.Value, len(v))
		for k, n := range v {
			values[k] = num(n)
		}
		return tftypes.NewValue(tftypes.Map{ElementType: tftypes.Number}, values)
	}

	cases := []struct {
		name        string
		newResource func() fwresource.Resource
		prior       map[string]tftypes.Value
		unknown     []string
	}{
		{
			name:        "channel",
			newResource: NewChannelResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccchannel"), "title": str("TF Acc Channel"),
				"enabled": boolean(false), "notes": str("keep"),
				"users": strList("admin"), "email": str("keep@example.com"),
			},
			unknown: []string{"enabled", "notes", "users", "email"},
		},
		{
			name:        "secret",
			newResource: NewSecretResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccsecret"), "title": str("TF Acc Secret"),
				"enabled": boolean(false), "notes": str("keep"),
				"value": str("s3cret"), "category": strList("keep"),
				"params": str(`[{"id":"keep"}]`),
			},
			unknown: []string{"enabled", "notes", "value", "category", "params"},
		},
		{
			name:        "alert",
			newResource: NewAlertResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccalert"), "title": str("TF Acc Alert"),
				"expression": str("cpu.currentLoad > 80"), "message": str("hot"),
				"enabled": boolean(false), "samples": num(5), "max_count": num(3),
				"notes": str("keep"), "monitor_id": str("cpu"),
				"groups": strList("keep"), "actions": str(`[{"type":"email"}]`),
			},
			unknown: []string{"enabled", "samples", "max_count", "notes", "monitor_id", "groups", "actions"},
		},
		{
			name:        "plugin",
			newResource: NewPluginResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccplugin"), "title": str("TF Acc Plugin"),
				"enabled": boolean(false), "type": str("event"),
				"command": str("sh"), "script_file": str("#!/bin/sh"),
				"uid": num(1000), "gid": num(1000), "params": str(`[]`),
				"groups": strList("keep"), "format": str("text"),
			},
			unknown: []string{"enabled", "command", "script_file", "uid", "gid", "params", "groups", "format"},
		},
		{
			name:        "event",
			newResource: NewEventResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccevent"), "title": str("TF Acc Event"),
				"enabled": boolean(false), "category": str("general"),
				"plugin": str("shellplug"), "targets": strList("local"),
				"params": str(`{"script":"x"}`), "fields": str(`[]`),
				"tags": strList("tfacceventtagz"), "algo": str("random"),
				"notes": str("keep"), "actions": str(`[]`), "limits": str(`[]`),
				"triggers": str(`[]`),
			},
			unknown: []string{"enabled", "category", "plugin", "targets", "params", "fields", "tags", "algo", "notes", "actions", "limits", "triggers"},
		},
		{
			name:        "api_key",
			newResource: NewAPIKeyResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccapikey"), "title": str("TF Acc Key"),
				"description": str("keep"), "active": boolean(false),
				"expires": num(123), "key": str("k"), "plain_key": str("p"),
				"privileges": numMap(map[string]int64{"admin": 1}),
				"roles":      strList("keep"),
			},
			unknown: []string{"description", "active", "expires", "privileges", "roles"},
		},
		{
			name:        "role",
			newResource: NewRoleResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccrole"), "title": str("TF Acc Role"),
				"enabled": boolean(false), "privileges": numMap(map[string]int64{"view": 1}),
				"categories": strList("keep"), "notes": str("keep"),
			},
			unknown: []string{"enabled", "privileges", "categories", "notes"},
		},
		{
			name:        "bucket",
			newResource: NewBucketResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccbucket"), "title": str("TF Acc Bucket"),
				"enabled": boolean(false), "icon": str("box"),
				"notes": str("keep"), "data": str(`{"a":1}`),
			},
			unknown: []string{"enabled", "icon", "notes", "data"},
		},
		{
			name:        "category",
			newResource: NewCategoryResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccategory"), "title": str("TF Acc Category"),
				"enabled": boolean(false), "icon": str("folder"),
				"color": str("blue"), "notes": str("keep"),
				"actions": str(`[]`), "limits": str(`[]`),
			},
			unknown: []string{"enabled", "icon", "color", "notes", "actions", "limits"},
		},
		{
			name:        "tag",
			newResource: NewTagResource,
			prior: map[string]tftypes.Value{
				"id": str("tfacctag"), "title": str("TF Acc Tag"),
				"enabled": boolean(false), "icon": str("tag"),
				"color": str("blue"), "notes": str("keep"),
			},
			unknown: []string{"enabled", "icon", "color", "notes"},
		},
		{
			name:        "webhook",
			newResource: NewWebhookResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccwebhook"), "title": str("TF Acc Webhook"),
				"enabled": boolean(false), "icon": str("web"),
				"url": str("https://example.com/hook"), "method": str("POST"),
				"headers": str(`[]`), "body": str("{}"), "timeout": num(10),
				"retries": num(2), "follow": boolean(true),
				"ssl_cert_bypass": boolean(true), "max_per_day": num(5),
				"notes": str("keep"),
			},
			unknown: []string{"enabled", "icon", "headers", "body", "timeout", "retries", "follow", "ssl_cert_bypass", "max_per_day", "notes"},
		},
		{
			name:        "group",
			newResource: NewGroupResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccgroup"), "title": str("TF Acc Group"),
				"enabled": boolean(false), "hostname_match": str(".*"),
				"sort_order": num(7), "alert_actions": str(`[]`),
				"notes": str("keep"), "max_jobs_per_server": num(3),
			},
			unknown: []string{"enabled", "sort_order", "max_jobs_per_server", "alert_actions", "notes"},
		},
		{
			name:        "monitor",
			newResource: NewMonitorResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccmonitor"), "title": str("TF Acc Monitor"),
				"source": str("cpu.currentLoad"), "data_type": str("float"),
				"display": boolean(false), "groups": strList("keep"),
				"suffix": str("%"), "min_vert_scale": num(9), "max_vert_scale": num(99),
				"data_match": str("keep"), "delta": boolean(true),
				"divide_time": boolean(true), "zero_min": num(1),
				"notes": str("keep"),
			},
			unknown: []string{"display", "groups", "suffix", "min_vert_scale", "max_vert_scale", "data_match", "delta", "divide_time", "zero_min", "notes"},
		},
		{
			name:        "workflow",
			newResource: NewWorkflowResource,
			prior: map[string]tftypes.Value{
				"id": str("tfaccworkflow"), "title": str("TF Acc Workflow"),
				"enabled": boolean(false), "category": str("general"),
				"tags": strList("keep"), "triggers": str(`[]`),
				"actions": str(`[]`), "limits": str(`[]`),
				"workflow": str(`{"nodes":[]}`),
			},
			unknown: []string{"enabled", "category", "tags", "triggers", "actions", "limits"},
		},
		{
			name:        "config",
			newResource: NewConfigResource,
			prior: map[string]tftypes.Value{
				"id":                    str(configSyntheticID),
				"job_universal_actions": str(`{"default":[],"workflow":[]}`),
			},
			unknown: []string{"job_universal_actions"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := tc.newResource()
			schemaResp := &fwresource.SchemaResponse{}
			res.Schema(ctx, fwresource.SchemaRequest{}, schemaResp)
			objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)

			build := func(unknown []string) tftypes.Value {
				values := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
				for name, attributeType := range objectType.AttributeTypes {
					values[name] = tftypes.NewValue(attributeType, nil)
				}
				for name, value := range tc.prior {
					values[name] = value
				}
				for _, name := range unknown {
					values[name] = tftypes.NewValue(objectType.AttributeTypes[name], tftypes.UnknownValue)
				}
				return tftypes.NewValue(objectType, values)
			}
			priorState := build(nil)
			plan := build(tc.unknown)

			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"code":0}`)
			}))
			defer server.Close()
			client, err := xyops.NewClient(server.URL, "test-key")
			if err != nil {
				t.Fatalf("failed to build the xyOps client: %s", err)
			}

			configured, ok := res.(fwresource.ResourceWithConfigure)
			if !ok {
				t.Fatalf("resource %T does not implement ResourceWithConfigure", res)
			}
			configureResp := &fwresource.ConfigureResponse{}
			configured.Configure(ctx, fwresource.ConfigureRequest{ProviderData: client}, configureResp)
			if configureResp.Diagnostics.HasError() {
				t.Fatalf("Configure failed: %s", configureResp.Diagnostics)
			}

			resp := &fwresource.UpdateResponse{
				State: tfsdk.State{Schema: schemaResp.Schema, Raw: priorState},
			}
			res.Update(ctx, fwresource.UpdateRequest{
				Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
				State: tfsdk.State{Schema: schemaResp.Schema, Raw: priorState},
			}, resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Update with unknown plan values failed: %s", resp.Diagnostics)
			}
			if calls.Load() != 0 {
				t.Errorf("Update sent %d API action(s), want none: an unknown attribute was never configured and the API keeps it", calls.Load())
			}
			if err := walkUnknown(resp.State.Raw); err != nil {
				t.Error(err)
			}
		})
	}
}

func walkUnknown(value tftypes.Value) error {
	return tftypes.Walk(value, func(step *tftypes.AttributePath, walked tftypes.Value) (bool, error) {
		if !walked.IsKnown() {
			return false, fmt.Errorf("the resulting state is not wholly known at %s", step)
		}
		return true, nil
	})
}
