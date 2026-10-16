package provider

import (
	"errors"
	"testing"

	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// The xyOps instance always has at least one server record: local self manages
// itself, so the data source never returns an empty total there.
func TestAccServersDataSource_search(t *testing.T) {
	testAccPreCheck(t)

	tfresource.Test(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: providerConfig() + `
data "xyops_servers" "probe" {
  query = "groups:main"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.xyops_servers.probe", tfjsonpath.New("total"), knownvalue.Int64Func(func(v int64) error {
						if v < 1 {
							return errors.New("expected at least one server record")
						}
						return nil
					})),
				},
			},
			{
				Config: providerConfig() + `
data "xyops_servers" "probe" {
  query = "os_distro:g594"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.xyops_servers.probe", tfjsonpath.New("total"), knownvalue.Int64Exact(0)),
				},
			},
		},
	})
}

// Jobs and alerts are runtime state; a quiet xyOps has none, so the plumbing
// check is that the search answers both attribues without error.
func TestAccJobsAndAlertsDataSources_empty(t *testing.T) {
	testAccPreCheck(t)

	tfresource.Test(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []tfresource.TestStep{
			{
				Config: providerConfig() + `
data "xyops_jobs" "probe" {
  query = "event:tfacc-no-such-event"
}
data "xyops_alerts" "probe" {
  query = "alert:tfacc-no-such-alert"
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.xyops_jobs.probe", tfjsonpath.New("total"), knownvalue.Int64Exact(0)),
					statecheck.ExpectKnownValue("data.xyops_alerts.probe", tfjsonpath.New("total"), knownvalue.Int64Exact(0)),
				},
			},
		},
	})
}
