package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestWorkflowNodeWithoutCoordinatesIsDiagnosed(t *testing.T) {
	var mu sync.Mutex
	var createCalls int
	var storedEvent map[string]any
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
			createCalls++
			storedEvent = payload
			mu.Unlock()
			payload["code"] = float64(0)
			writeJSONTestResponse(w, map[string]any{"code": 0, "event": payload})
		case "/api/app/get_event/v1":
			mu.Lock()
			defer mu.Unlock()
			if storedEvent == nil {
				_, _ = io.WriteString(w, `{"code":"notfound","description":"missing"}`)
				return
			}
			writeJSONTestResponse(w, map[string]any{"code": 0, "event": storedEvent})
		case "/api/app/delete_event/v1":
			mu.Lock()
			storedEvent = nil
			mu.Unlock()
			_, _ = io.WriteString(w, `{"code":0}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	// Each workflow node must carry x and y: the API cannot place a node
	// without coordinates, so the provider's validation runs before asking
	// the server to build a workflow it could never render.
	nodeWithoutY := func(y string) string {
		workflow := `{"nodes":[{"id":"start","type":"event","event":"unit-plugin","x":10` + y + `}],"connections":[]}`
		return providerConfig() + `
resource "xyops_workflow" "test" {
  id       = "unit-workflow"
  title    = "Unit Workflow"
  workflow = jsonencode(` + workflow + `)
}
`
	}
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config:      nodeWithoutY(``),
				ExpectError: regexp.MustCompile(`Missing y`),
			},
			{
				Config:      providerConfig() + "\nresource \"xyops_workflow\" \"test\" {\n  id       = \"unit-workflow\"\n  title    = \"Unit Workflow\"\n  workflow = jsonencode({\"nodes\":[{\"id\":\"start\",\"type\":\"event\",\"event\":\"unit-plugin\",\"y\":10}],\"connections\":[]})\n}",
				ExpectError: regexp.MustCompile(`Missing x`),
			},
			{
				Config: nodeWithoutY(`,"y":10`),
			},
		},
	})
	mu.Lock()
	defer mu.Unlock()
	if createCalls != 1 {
		t.Errorf("create_event calls = %d, want 1 (only the valid node)", createCalls)
	}
}

func TestBucketResourceDataShallowMerge(t *testing.T) {
	var mu sync.Mutex
	var updatePayload map[string]any
	title := "Unit Bucket"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/app/search_alerts/v1":
			_, _ = io.WriteString(w, `{"code":0}`)
		case "/api/app/create_bucket/v1":
			_, _ = io.WriteString(w, `{"code":0,"bucket":{"id":"unit-bucket","title":"Unit Bucket","enabled":true,"data":{"key":"v1"},"notes":""}}`)
		case "/api/app/get_bucket/v1":
			_, _ = io.WriteString(w, `{"code":0,"bucket":{"id":"unit-bucket","title":"`+title+`","enabled":true,"data":{"key":"v1"},"notes":""}}`)
		case "/api/app/update_bucket/v1":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read request body: %s", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("decode request body: %s", err)
			}
			mu.Lock()
			if newTitle, ok := payload["title"].(string); ok {
				title = newTitle
			}
			updatePayload = payload
			mu.Unlock()
			_, _ = io.WriteString(w, `{"code":0}`)
		case "/api/app/delete_bucket/v1":
			_, _ = io.WriteString(w, `{"code":0}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	// After create, update only the title: the shallow-merge must not rewrite
	// the data attribute the API reported and the config does not change.
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_bucket" "test" {
  id    = "unit-bucket"
  title = "Unit Bucket"
  data  = jsonencode({ key = "v1" })
}
`,
				Check: tfresource.TestCheckResourceAttr("xyops_bucket.test", "data", `{"key":"v1"}`),
			},
			{
				Config: providerConfig() + `
resource "xyops_bucket" "test" {
  id    = "unit-bucket"
  title = "Unit Bucket Moved"
  data  = jsonencode({ key = "v1" })
}
`,
				Check: tfresource.TestCheckResourceAttr("xyops_bucket.test", "title", "Unit Bucket Moved"),
			},
		},
	})
	mu.Lock()
	defer mu.Unlock()
	if len(updatePayload) != 2 || !hasIDAndTitleOnly(updatePayload) {
		t.Errorf("update payload = %#v, want only id and title", updatePayload)
	}
	if data, ok := updatePayload["data"]; ok {
		t.Errorf("update payload rewrote data = %#v, want only the changed field", data)
	}
}

func hasIDAndTitleOnly(payload map[string]any) bool {
	_, hasID := payload["id"]
	_, hasTitle := payload["title"]
	return hasID && hasTitle && len(payload) == 2
}
