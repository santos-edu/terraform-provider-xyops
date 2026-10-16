package provider

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func searchTestServer(t *testing.T, requests *[]*http.Request) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("request method = %s, want GET", r.Method)
		}
		*requests = append(*requests, r)
		switch r.URL.Path {
		case "/api/app/search_servers/v1":
			_, _ = io.WriteString(w, `{"code":0,"rows":[{"id":"srv_main","hostname":"host-1","ip":"10.0.0.1","enabled":true,"autoGroup":true,"groups":["main"],"keywords":"host-1,main","notes":"probe","created":100,"modified":200},{"id":"srv_drop","hostname":"host-2","ip":"10.0.0.2","enabled":false,"autoGroup":false,"groups":[],"keywords":"","notes":"","created":1,"modified":2}],"list":{"length":9}}`)
		case "/api/app/search_jobs/v1":
			_, _ = io.WriteString(w, `{"code":0,"rows":[{"id":"job_1","event":"ev_1","event_title":"Probe Event","server":"srv_main","groups":["main"],"category":"general","plugin":"shellplug","state":"complete","started":5.5,"completed":6,"elapsed":0.5,"code":0,"description":"","tags":["_success"]}],"list":{"length":1}}`)
		case "/api/app/search_alerts/v1":
			_, _ = io.WriteString(w, `{"code":0,"rows":[{"id":"al_1","alert":"load_avg_high","server":"srv_main","groups":["main"],"date":7,"exp":"true","message":"fired","count":2,"active":true,"notified":true,"modified":7.25,"jobs":[],"tickets":[]}],"list":{"length":1}}`)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func readSearchQuery(t *testing.T, requests []*http.Request, path string) url.Values {
	t.Helper()
	for _, r := range requests {
		// Skip the ValidateKey probe, which uses the same search_alerts
		// endpoint with a query and limit that no data source sends.
		if r.URL.Path == path && r.URL.Query().Get("query") != "alert:_xyops_provider_probe" {
			values, err := url.ParseQuery(r.URL.RawQuery)
			if err != nil {
				t.Fatalf("parse query %q: %s", r.URL.RawQuery, err)
			}
			return values
		}
	}
	t.Fatalf("no request captured for %s", path)
	return nil
}

func TestSearchDataSources_readAndDefaults(t *testing.T) {
	var requests []*http.Request
	server := searchTestServer(t, &requests)
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	config := providerConfig() + `
data "xyops_servers" "probe" {
  query = "groups:main"
}
data "xyops_jobs" "probe" {
  query = "event:ev_1"
}
data "xyops_alerts" "probe" {}
`
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.xyops_servers.probe", tfjsonpath.New("total"), knownvalue.Int64Exact(9)),
					statecheck.ExpectKnownValue("data.xyops_jobs.probe", tfjsonpath.New("total"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("data.xyops_alerts.probe", tfjsonpath.New("total"), knownvalue.Int64Exact(1)),
				},
			},
		},
	})

	if len(requests) > 16 {
		t.Fatalf("requests = %d, want at most one cycle", len(requests))
	}
	first := readSearchQuery(t, requests, "/api/app/search_servers/v1")
	if first.Get("query") != "groups:main" || first.Get("limit") != "100" || first.Get("offset") != "0" {
		t.Errorf("servers query = %v, want query=groups:main limit=100 offset=0", first)
	}
	jobs := readSearchQuery(t, requests, "/api/app/search_jobs/v1")
	if jobs.Get("query") != "event:ev_1" {
		t.Errorf("jobs query = %v, want query=event:ev_1", jobs)
	}
	alerts := readSearchQuery(t, requests, "/api/app/search_alerts/v1")
	if alerts.Get("query") != "*" {
		t.Errorf("alerts query = %v, want query=*", alerts)
	}
}

func TestSearchDataSources_rowMapping(t *testing.T) {
	var requests []*http.Request
	server := searchTestServer(t, &requests)
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	config := providerConfig() + `
data "xyops_servers" "probe" {}
`
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.xyops_servers.probe",
						tfjsonpath.New("servers").AtSliceIndex(0).AtMapKey("id"),
						knownvalue.StringExact("srv_main"),
					),
					statecheck.ExpectKnownValue(
						"data.xyops_servers.probe",
						tfjsonpath.New("servers").AtSliceIndex(0).AtMapKey("groups"),
						knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("main")}),
					),
					statecheck.ExpectKnownValue(
						"data.xyops_servers.probe",
						tfjsonpath.New("servers").AtSliceIndex(1).AtMapKey("groups"),
						knownvalue.Null(),
					),
				},
			},
		},
	})
}

func TestSearchDataSource_apiErrorSurfaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/app/search_alerts/v1":
			_, _ = io.WriteString(w, `{"code":0}`)
		case "/api/app/search_servers/v1":
			_, _ = io.WriteString(w, `{"code":"db","description":"Failed DB search"}`)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("XYOPS_ENDPOINT", server.URL)
	t.Setenv("XYOPS_API_KEY", "test-key")

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config:      providerConfig() + "\ndata \"xyops_servers\" \"probe\" {}\n",
				ExpectError: regexp.MustCompile(`Search xyOps servers failed`),
			},
		},
	})
}
