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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestTagResourceOptionalValuesAndShallowMerge(t *testing.T) {
	server, updateBodies := newTagContractServer(t)
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	configWithoutNotes := func(id string) string {
		return providerConfig() + `
resource "xyops_tag" "test" {
  id    = "` + id + `"
  title = "Unit Tag"
}
`
	}
	configWithNotes := providerConfig() + `
resource "xyops_tag" "test" {
  id    = "unit-tag"
  title = "Unit Tag"
  notes = "temporary"
}
`
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: configWithoutNotes("unit-tag"),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckNoResourceAttr("xyops_tag.test", "notes"),
				),
			},
			{
				Config: configWithNotes,
				Check:  tfresource.TestCheckResourceAttr("xyops_tag.test", "notes", "temporary"),
			},
			{
				Config: configWithoutNotes("unit-tag"),
				Check:  tfresource.TestCheckNoResourceAttr("xyops_tag.test", "notes"),
			},
			{
				Config: configWithoutNotes("unit-tag"),
				Check:  tfresource.TestCheckNoResourceAttr("xyops_tag.test", "notes"),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: configWithoutNotes("unit-tag-next"),
				Check:  tfresource.TestCheckResourceAttr("xyops_tag.test", "id", "unit-tag-next"),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("xyops_tag.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
			},
		},
	})

	wantUpdates := []map[string]any{
		{"id": "unit-tag", "notes": "temporary"},
		{"id": "unit-tag", "notes": ""},
	}
	if !reflect.DeepEqual(*updateBodies, wantUpdates) {
		t.Errorf("update payloads = %#v, want %#v", *updateBodies, wantUpdates)
	}
}

func TestTagResourceImportMissingIDReturnsError(t *testing.T) {
	server, _ := newTagContractServer(t)
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_tag" "test" {
  id    = "unit-tag"
  title = "Unit Tag"
}
`,
			},
			{
				ResourceName:  "xyops_tag.test",
				ImportState:   true,
				ImportStateId: "missing-tag",
				ExpectError:   regexp.MustCompile(`(?s).+`),
			},
		},
	})
}

type tagContractRecord struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Enabled bool   `json:"enabled"`
	Icon    string `json:"icon"`
	Color   string `json:"color"`
	Notes   string `json:"notes"`
}

func newTagContractServer(t *testing.T) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var mu sync.Mutex
	tags := make(map[string]tagContractRecord)
	updateBodies := []map[string]any{}
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
		case "/api/app/create_tag/v1":
			id, _ := input["id"].(string)
			tag := tagContractRecord{ID: id, Enabled: true}
			if value, ok := input["title"].(string); ok {
				tag.Title = value
			}
			if value, ok := input["enabled"].(bool); ok {
				tag.Enabled = value
			}
			tags[id] = tag
			writeJSONTestResponse(w, map[string]any{"code": 0, "tag": tag})
		case "/api/app/get_tag/v1":
			id, _ := input["id"].(string)
			tag, ok := tags[id]
			if !ok {
				writeJSONTestResponse(w, map[string]any{"code": "notfound", "description": "Tag not found"})
				return
			}
			writeJSONTestResponse(w, map[string]any{"code": 0, "tag": tag})
		case "/api/app/update_tag/v1":
			updateBodies = append(updateBodies, input)
			id, _ := input["id"].(string)
			tag, ok := tags[id]
			if !ok {
				writeJSONTestResponse(w, map[string]any{"code": "notfound", "description": "Tag not found"})
				return
			}
			if value, ok := input["title"].(string); ok {
				tag.Title = value
			}
			if value, ok := input["notes"].(string); ok {
				tag.Notes = value
			}
			tags[id] = tag
			writeJSONTestResponse(w, map[string]any{"code": 0})
		case "/api/app/delete_tag/v1":
			id, _ := input["id"].(string)
			delete(tags, id)
			writeJSONTestResponse(w, map[string]any{"code": 0})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return server, &updateBodies
}

func writeJSONTestResponse(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
