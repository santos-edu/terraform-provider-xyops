package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccMonitorResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_monitor" "test" {
  id             = "tfaccmonitor"
  title          = "TF Acc Monitor"
  source         = "cpu.currentLoad"
  data_type      = "float"
  display        = true
  groups         = []
  suffix         = "%"
  min_vert_scale = 100
  max_vert_scale = 321
  data_match     = ""
  delta          = true
  divide_time    = true
  zero_min       = 0
  notes          = "Managed by Terraform"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_monitor.test", "id", "tfaccmonitor"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "title", "TF Acc Monitor"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "source", "cpu.currentLoad"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "data_type", "float"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "display", "true"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "suffix", "%"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "min_vert_scale", "100"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "max_vert_scale", "321"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "delta", "true"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "divide_time", "true"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "zero_min", "0"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "notes", "Managed by Terraform"),
				),
			},
			{
				Config: providerConfig() + `
resource "xyops_monitor" "test" {
  id             = "tfaccmonitor"
  title          = "TF Acc Monitor Updated"
  source         = "cpu.currentLoad"
  data_type      = "float"
  display        = true
  groups         = []
  suffix         = "%"
  min_vert_scale = 100
  max_vert_scale = 400
  data_match     = ""
  delta          = true
  divide_time    = true
  zero_min       = 1
  notes          = "Managed by Terraform"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_monitor.test", "id", "tfaccmonitor"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "title", "TF Acc Monitor Updated"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "max_vert_scale", "400"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "zero_min", "1"),
				),
			},
			{
				ResourceName:            "xyops_monitor.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"data_match"},
			},
		},
	})
}

func TestAccMonitorResource_groupsOrder(t *testing.T) {
	unsortedConfig := providerConfig() + `
resource "xyops_monitor" "test" {
  id             = "tfaccmonitor"
  title          = "TF Acc Monitor Groups Order"
  source         = "cpu.currentLoad"
  data_type      = "float"
  display        = true
  groups         = ["zgroup", "agroup"]
  suffix         = "%"
  min_vert_scale = 100
  data_match     = ""
  delta          = true
  divide_time    = true
  zero_min       = 1
}
`
	sortedConfig := providerConfig() + `
resource "xyops_monitor" "test" {
  id             = "tfaccmonitor"
  title          = "TF Acc Monitor Groups Order"
  source         = "cpu.currentLoad"
  data_type      = "float"
  display        = true
  groups         = ["agroup", "zgroup"]
  suffix         = "%"
  min_vert_scale = 100
  data_match     = ""
  delta          = true
  divide_time    = true
  zero_min       = 1
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.#", "2"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.0", "zgroup"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.1", "agroup"),
				),
			},
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.#", "2"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.0", "zgroup"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.1", "agroup"),
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
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.#", "2"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.0", "agroup"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.1", "zgroup"),
				),
			},
			{
				Config: sortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.#", "2"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.0", "agroup"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.1", "zgroup"),
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

func TestAccMonitorResource_omittedComputed(t *testing.T) {
	config := providerConfig() + `
resource "xyops_monitor" "test" {
  id        = "tfaccmonitor"
  title     = "TF Acc Monitor Omitted"
  source    = "cpu.currentLoad"
  data_type = "float"
}
`
	updatedConfig := providerConfig() + `
resource "xyops_monitor" "test" {
  id        = "tfaccmonitor"
  title     = "TF Acc Monitor Omitted Updated"
  source    = "cpu.currentLoad"
  data_type = "float"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_monitor.test", "id", "tfaccmonitor"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.#", "0"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "data_match", ""),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_monitor.test", "title", "TF Acc Monitor Omitted Updated"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.#", "0"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "data_match", ""),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_monitor.test", "groups.#", "0"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "data_match", ""),
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

func TestAccMonitorResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_monitor" "test" {
  id             = "tfaccmonitor"
  title          = "TF Acc Monitor Drift"
  source         = "cpu.currentLoad"
  data_type      = "float"
  display        = true
  groups         = []
  suffix         = "%"
  min_vert_scale = 100
  max_vert_scale = 400
  data_match     = ""
  delta          = false
  divide_time    = false
  zero_min       = 0
  notes          = "Managed by Terraform"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_monitor.test", "id", "tfaccmonitor"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_monitor", map[string]string{"id": testAccMonitorID()}, nil)
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
					resource.TestCheckResourceAttr("xyops_monitor.test", "id", "tfaccmonitor"),
					resource.TestCheckResourceAttr("xyops_monitor.test", "title", "TF Acc Monitor Drift"),
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
