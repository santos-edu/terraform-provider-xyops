package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccGroupResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_group" "test" {
  id                  = "tfaccgroup"
  title               = "TF Acc Group"
  enabled             = true
  hostname_match      = ".*"
  sort_order          = 42
  alert_actions       = "[]"
  notes               = "Managed by Terraform"
  max_jobs_per_server = 0
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_group.test", "id", "tfaccgroup"),
					resource.TestCheckResourceAttr("xyops_group.test", "title", "TF Acc Group"),
					resource.TestCheckResourceAttr("xyops_group.test", "enabled", "true"),
					resource.TestCheckResourceAttr("xyops_group.test", "hostname_match", ".*"),
					resource.TestCheckResourceAttr("xyops_group.test", "sort_order", "42"),
					resource.TestCheckResourceAttr("xyops_group.test", "alert_actions", "[]"),
					resource.TestCheckResourceAttr("xyops_group.test", "notes", "Managed by Terraform"),
					resource.TestCheckResourceAttr("xyops_group.test", "max_jobs_per_server", "0"),
				),
			},
			{
				ResourceName:            "xyops_group.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"alert_actions"},
			},
			{
				Config: providerConfig() + `
resource "xyops_group" "test" {
  id                  = "tfaccgroup"
  title               = "TF Acc Group Updated"
  enabled             = true
  hostname_match      = ".*"
  sort_order          = 43
  alert_actions       = "[]"
  notes               = "Managed by Terraform"
  max_jobs_per_server = 0
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_group.test", "id", "tfaccgroup"),
					resource.TestCheckResourceAttr("xyops_group.test", "title", "TF Acc Group Updated"),
					resource.TestCheckResourceAttr("xyops_group.test", "sort_order", "43"),
				),
			},
		},
	})
}

func TestAccGroupResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_group" "test" {
  id                  = "tfaccgroup"
  title               = "TF Acc Group Drift"
  enabled             = true
  hostname_match      = ".*"
  alert_actions       = "[]"
  notes               = "Managed by Terraform"
  max_jobs_per_server = 0
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_group.test", "id", "tfaccgroup"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_group", map[string]string{"id": testAccGroupID()}, nil)
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
			},
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_group.test", "id", "tfaccgroup"),
					resource.TestCheckResourceAttr("xyops_group.test", "title", "TF Acc Group Drift"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccGroupID() string { return "tfaccgroup" }
