package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"sync"
	"testing"

	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestPluginResourceMonitorRequiresCommand(t *testing.T) {
	var createCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/app/search_alerts/v1":
			_, _ = io.WriteString(w, `{"code":0}`)
		case "/api/app/create_plugin/v1":
			createCalls++
			_, _ = io.WriteString(w, `{"code":"api","description":"Missing parameter: command"}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	// The API rejects a monitor plugin without command; the provider surfaces
	// the error and does not create partial state.
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_plugin" "test" {
  id          = "unit-plugin"
  title       = "Unit Plugin"
  type        = "monitor"
  script_file = "#!/bin/sh\necho data"
}
`,
				ExpectError: regexp.MustCompile(`Missing parameter: command`),
			},
		},
	})
	if createCalls != 1 {
		t.Errorf("create_plugin calls = %d, want 1", createCalls)
	}
}

func TestWebhookMethodEnumOnPlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/app/search_alerts/v1":
			_, _ = io.WriteString(w, `{"code":0}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	// xyOps UI restricts web hook methods to GET, HEAD, POST, PUT and DELETE;
	// the schema rejects anything else before the create is attempted.
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_webhook" "test" {
  id    = "unit-webhook"
  title = "Unit Webhook"
  url   = "https://example.com/hook"
  method = "BREW"
}
`,
				ExpectError: regexp.MustCompile(`Value must be one of: GET, HEAD, POST, PUT, DELETE`),
			},
		},
	})
}

func TestEventResourceCreateValidationOnPlan(t *testing.T) {
	var mu sync.Mutex
	responses := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/app/search_alerts/v1":
			_, _ = io.WriteString(w, `{"code":0}`)
		case "/api/app/create_event/v1":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read request body: %s", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("decode request body: %s", err)
			}
			mu.Lock()
			defer mu.Unlock()
			if targets, ok := payload["targets"].([]any); ok && len(targets) == 0 {
				responses["no-targets"]++
				_, _ = io.WriteString(w, `{"code":"api","description":"Event has no targets specified."}`)
				return
			}
			if _, ok := payload["algo"]; !ok {
				responses["no-algo"]++
				_, _ = io.WriteString(w, `{"code":"api","description":"Event has no algo specified."}`)
				return
			}
			t.Errorf("create_event expected to fail, got payload %#v", payload)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	// The API rejects these creates. The provider surfaces the API error and
	// does not store state.
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_event" "test" {
  id       = "unit-event"
  title    = "Unit Event"
  plugin   = "unit-plugin"
  targets  = []
  algo     = "random"
  category = "general"
}
`,
				ExpectError: regexp.MustCompile(`Event has no targets specified`),
			},
			{
				Config: providerConfig() + `
resource "xyops_event" "test" {
  id       = "unit-event"
  title    = "Unit Event"
  plugin   = "unit-plugin"
  targets  = ["local"]
  category = "general"
}
`,
				ExpectError: regexp.MustCompile(`Event has no algo specified`),
			},
		},
	})
	mu.Lock()
	defer mu.Unlock()
	if responses["no-targets"] != 1 || responses["no-algo"] != 1 {
		t.Errorf("create_event failure counts = %#v, want one of each", responses)
	}
}

func TestEventCreatePayloadValidation(t *testing.T) {
	var mutex sync.Mutex
	var captured map[string]any
	var gotPayload bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/app/search_alerts/v1":
			_, _ = io.WriteString(w, `{"code":0}`)
			return
		case "/api/app/create_event/v1":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read create_event request: %s", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("decode create_event request: %s", err)
			}
			gotPayload = true
			mutex.Lock()
			captured = payload
			mutex.Unlock()
			_, _ = io.WriteString(w, `{"code":"api","description":"Malformed parameter value: plugin"}`)
			return
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	config := providerConfig() + `
resource "xyops_event" "test" {
  id       = "unit-event"
  title    = "Unit Event"
  plugin   = "unit-plugin"
  targets  = ["local"]
  algo     = "random"
  category = "general"
}
`
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile("Malformed parameter value: plugin"),
			},
		},
	})

	mutex.Lock()
	defer mutex.Unlock()
	if !gotPayload {
		t.Fatal("create_event was not called")
	}
	want := []any{"local"}
	if got, ok := captured["targets"].([]any); !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("create_event targets = %#v, want %#v", captured["targets"], want)
	}
	if got := captured["algo"]; got != "random" {
		t.Errorf("create_event algo = %#v, want random", got)
	}
}
