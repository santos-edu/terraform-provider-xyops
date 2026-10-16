package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAPIKeyResourceCreateAndUpdateContract(t *testing.T) {
	var mu sync.Mutex
	var apiKey map[string]any
	var createPayload, updatePayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/app/search_alerts/v1" {
			writeJSONTestResponse(w, map[string]any{"code": 0})
			return
		}
		var input map[string]any
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read %s request: %s", r.URL.Path, err)
			}
			if err := json.Unmarshal(body, &input); err != nil {
				t.Errorf("decode %s request: %s", r.URL.Path, err)
			}
		}

		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/api/app/create_api_key/v1":
			createPayload = input
			apiKey = map[string]any{
				"id":          input["id"],
				"title":       input["title"],
				"description": input["description"],
				"active":      input["active"],
				"expires":     input["expires"],
				"privileges":  input["privileges"],
				"roles":       input["roles"],
				"key":         "hashed-key",
			}
			writeJSONTestResponse(w, map[string]any{"code": 0, "api_key": apiKey, "plain_key": "plain-key"})
		case "/api/app/get_api_key/v1":
			responseKey := make(map[string]any, len(apiKey))
			for key, value := range apiKey {
				if key != "key" {
					responseKey[key] = value
				}
			}
			writeJSONTestResponse(w, map[string]any{"code": 0, "api_key": responseKey})
		case "/api/app/update_api_key/v1":
			updatePayload = input
			for key, value := range input {
				if key != "id" {
					apiKey[key] = value
				}
			}
			writeJSONTestResponse(w, map[string]any{"code": 0})
		case "/api/app/delete_api_key/v1":
			apiKey = nil
			writeJSONTestResponse(w, map[string]any{"code": 0})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	initialConfig := providerConfig() + `
resource "xyops_api_key" "test" {
  id         = "unit-api-key"
  title      = "Unit API Key"
  active     = false
  expires    = 1700000123
  privileges = { admin = 1 }
  roles      = []
}
`
	updatedConfig := providerConfig() + `
resource "xyops_api_key" "test" {
  id         = "unit-api-key"
  title      = "Unit API Key Updated"
  active     = false
  expires    = 1700000123
  privileges = { admin = 1 }
  roles      = []
}
`
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: initialConfig,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("xyops_api_key.test", "active", "false"),
					tfresource.TestCheckResourceAttr("xyops_api_key.test", "expires", "1700000123"),
					tfresource.TestCheckResourceAttr("xyops_api_key.test", "privileges.admin", "1"),
					tfresource.TestCheckResourceAttr("xyops_api_key.test", "key", "hashed-key"),
					tfresource.TestCheckResourceAttr("xyops_api_key.test", "plain_key", "plain-key"),
				),
			},
			{
				Config: updatedConfig,
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr("xyops_api_key.test", "title", "Unit API Key Updated"),
					tfresource.TestCheckResourceAttr("xyops_api_key.test", "active", "false"),
					tfresource.TestCheckResourceAttr("xyops_api_key.test", "expires", "1700000123"),
					tfresource.TestCheckResourceAttr("xyops_api_key.test", "privileges.admin", "1"),
					tfresource.TestCheckResourceAttr("xyops_api_key.test", "key", "hashed-key"),
					tfresource.TestCheckResourceAttr("xyops_api_key.test", "plain_key", "plain-key"),
				),
			},
			{
				Config: updatedConfig,
				ConfigPlanChecks: tfresource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})

	wantCreate := map[string]any{
		"id":         "unit-api-key",
		"title":      "Unit API Key",
		"active":     false,
		"expires":    float64(1700000123),
		"privileges": map[string]any{"admin": float64(1)},
		"roles":      []any{},
	}
	if !reflect.DeepEqual(createPayload, wantCreate) {
		t.Errorf("create payload = %#v, want %#v", createPayload, wantCreate)
	}
	wantUpdate := map[string]any{"id": "unit-api-key", "title": "Unit API Key Updated"}
	if !reflect.DeepEqual(updatePayload, wantUpdate) {
		t.Errorf("update payload = %#v, want %#v", updatePayload, wantUpdate)
	}
}
