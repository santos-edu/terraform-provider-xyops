package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccTagResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_tag" "test" {
  id      = "tfacctag"
  title   = "TF Acc Tag"
  enabled = true
  icon    = "tag"
  color   = "blue"
  notes   = "Managed by Terraform"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_tag.test", "id", "tfacctag"),
					resource.TestCheckResourceAttr("xyops_tag.test", "title", "TF Acc Tag"),
					resource.TestCheckResourceAttr("xyops_tag.test", "enabled", "true"),
					resource.TestCheckResourceAttr("xyops_tag.test", "icon", "tag"),
					resource.TestCheckResourceAttr("xyops_tag.test", "color", "blue"),
					resource.TestCheckResourceAttr("xyops_tag.test", "notes", "Managed by Terraform"),
				),
			},
			{
				ResourceName:      "xyops_tag.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: providerConfig() + `
resource "xyops_tag" "test" {
  id      = "tfacctag"
  title   = "TF Acc Tag Updated"
  enabled = true
  icon    = "tag"
  color   = "blue"
  notes   = "Managed by Terraform"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_tag.test", "id", "tfacctag"),
					resource.TestCheckResourceAttr("xyops_tag.test", "title", "TF Acc Tag Updated"),
				),
			},
		},
	})
}

func TestAccTagResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_tag" "test" {
  id      = "tfacctag"
  title   = "TF Acc Tag Drift"
  enabled = true
  icon    = "tag"
  color   = "blue"
  notes   = "Managed by Terraform"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_tag.test", "id", "tfacctag"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_tag", map[string]string{"id": testAccTagID()}, nil)
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
					resource.TestCheckResourceAttr("xyops_tag.test", "id", "tfacctag"),
					resource.TestCheckResourceAttr("xyops_tag.test", "title", "TF Acc Tag Drift"),
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

func testAccTagID() string { return "tfacctag" }
