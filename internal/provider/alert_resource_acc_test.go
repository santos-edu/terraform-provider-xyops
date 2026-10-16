package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccAlertResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_alert" "test" {
  id         = "tfaccalert"
  title      = "TF Acc Alert"
  expression = "cpu.currentLoad > 80"
  message    = "CPU load is {{cpu.currentLoad}}%"
  enabled    = true
  samples    = 1
  max_count  = 1
  notes      = "Managed by Terraform"
  groups     = []
  actions    = "[]"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_alert.test", "id", "tfaccalert"),
					resource.TestCheckResourceAttr("xyops_alert.test", "title", "TF Acc Alert"),
					resource.TestCheckResourceAttr("xyops_alert.test", "expression", "cpu.currentLoad > 80"),
					resource.TestCheckResourceAttr("xyops_alert.test", "message", "CPU load is {{cpu.currentLoad}}%"),
					resource.TestCheckResourceAttr("xyops_alert.test", "enabled", "true"),
					resource.TestCheckResourceAttr("xyops_alert.test", "samples", "1"),
					resource.TestCheckResourceAttr("xyops_alert.test", "max_count", "1"),
					resource.TestCheckResourceAttr("xyops_alert.test", "notes", "Managed by Terraform"),
					resource.TestCheckResourceAttr("xyops_alert.test", "actions", "[]"),
				),
			},
			{
				Config: providerConfig() + `
resource "xyops_alert" "test" {
  id         = "tfaccalert"
  title      = "TF Acc Alert Updated"
  expression = "cpu.currentLoad > 80"
  message    = "CPU load is {{cpu.currentLoad}}%"
  enabled    = true
  samples    = 1
  max_count  = 3
  notes      = "Managed by Terraform"
  groups     = []
  actions    = "[]"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_alert.test", "id", "tfaccalert"),
					resource.TestCheckResourceAttr("xyops_alert.test", "title", "TF Acc Alert Updated"),
					resource.TestCheckResourceAttr("xyops_alert.test", "max_count", "3"),
				),
			},
			{
				ResourceName:            "xyops_alert.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"actions"},
			},
		},
	})
}

func TestAccAlertResource_groupsOrder(t *testing.T) {
	unsortedConfig := providerConfig() + `
resource "xyops_alert" "test" {
  id         = "tfaccalert"
  title      = "TF Acc Alert Groups Order"
  expression = "cpu.currentLoad > 80"
  message    = "CPU load is {{cpu.currentLoad}}%"
  enabled    = true
  samples    = 1
  max_count  = 1
  groups     = ["zgroup", "agroup"]
  actions    = "[]"
}
`
	sortedConfig := providerConfig() + `
resource "xyops_alert" "test" {
  id         = "tfaccalert"
  title      = "TF Acc Alert Groups Order"
  expression = "cpu.currentLoad > 80"
  message    = "CPU load is {{cpu.currentLoad}}%"
  enabled    = true
  samples    = 1
  max_count  = 1
  groups     = ["agroup", "zgroup"]
  actions    = "[]"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.#", "2"),
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.0", "zgroup"),
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.1", "agroup"),
				),
			},
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.#", "2"),
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.0", "zgroup"),
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.1", "agroup"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: sortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.#", "2"),
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.0", "agroup"),
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.1", "zgroup"),
				),
			},
			{
				Config: sortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.#", "2"),
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.0", "agroup"),
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.1", "zgroup"),
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

func TestAccAlertResource_omittedComputed(t *testing.T) {
	config := providerConfig() + `
resource "xyops_alert" "test" {
  id         = "tfaccalert"
  title      = "TF Acc Alert Omitted"
  expression = "cpu.currentLoad > 80"
  message    = "CPU load is {{cpu.currentLoad}}%"
}
`
	updatedConfig := providerConfig() + `
resource "xyops_alert" "test" {
  id         = "tfaccalert"
  title      = "TF Acc Alert Omitted Updated"
  expression = "cpu.currentLoad > 80"
  message    = "CPU load is {{cpu.currentLoad}}%"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_alert.test", "id", "tfaccalert"),
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.#", "0"),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_alert.test", "title", "TF Acc Alert Omitted Updated"),
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.#", "0"),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_alert.test", "groups.#", "0"),
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

func TestAccAlertResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_alert" "test" {
  id         = "tfaccalert"
  title      = "TF Acc Alert Drift"
  expression = "cpu.currentLoad > 80"
  message    = "CPU load is {{cpu.currentLoad}}%"
  enabled    = true
  samples    = 1
  max_count  = 1
  notes      = "Managed by Terraform"
  groups     = []
  actions    = "[]"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_alert.test", "id", "tfaccalert"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_alert", map[string]string{"id": testAccAlertID()}, nil)
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
					resource.TestCheckResourceAttr("xyops_alert.test", "id", "tfaccalert"),
					resource.TestCheckResourceAttr("xyops_alert.test", "title", "TF Acc Alert Drift"),
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
