package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccConfigResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_config" "test" {
  job_universal_actions = jsonencode({
    default = [{ type = "web_hook", enabled = true, web_hook = "example_hook" }]
    workflow = []
  })
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_config.test", "id", "global"),
					resource.TestCheckResourceAttr("xyops_config.test", "job_universal_actions", `{"default":[{"enabled":true,"type":"web_hook","web_hook":"example_hook"}],"workflow":[]}`),
				),
			},
			{
				Config: providerConfig() + `
resource "xyops_config" "test" {
  job_universal_actions = jsonencode({
    default  = []
    workflow = [{ type = "web_hook", enabled = true, web_hook = "workflow_hook" }]
  })
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_config.test", "id", "global"),
					resource.TestCheckResourceAttr("xyops_config.test", "job_universal_actions", `{"default":[],"workflow":[{"enabled":true,"type":"web_hook","web_hook":"workflow_hook"}]}`),
				),
			},
		},
	})
}

func TestAccConfigResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_config" "test" {
  job_universal_actions = jsonencode({
    default  = [{ type = "web_hook", enabled = true, web_hook = "drift_hook" }]
    workflow = []
  })
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_config.test", "id", "global"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("admin_update_config", map[string]any{
						"job_universal_actions": map[string]any{
							"default":  []any{},
							"workflow": []any{},
						},
					}, nil)
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
					resource.TestCheckResourceAttr("xyops_config.test", "id", "global"),
					resource.TestCheckResourceAttr("xyops_config.test", "job_universal_actions", `{"default":[{"enabled":true,"type":"web_hook","web_hook":"drift_hook"}],"workflow":[]}`),
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
