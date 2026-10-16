package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccCategoryResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_category" "test" {
  id      = "tfacccategory"
  title   = "TF Acc Category"
  enabled = true
  icon    = "folder"
  color   = "blue"
  notes   = "Managed by Terraform"
  actions = "[]"
  limits  = "[]"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_category.test", "id", "tfacccategory"),
					resource.TestCheckResourceAttr("xyops_category.test", "title", "TF Acc Category"),
					resource.TestCheckResourceAttr("xyops_category.test", "enabled", "true"),
					resource.TestCheckResourceAttr("xyops_category.test", "icon", "folder"),
					resource.TestCheckResourceAttr("xyops_category.test", "color", "blue"),
					resource.TestCheckResourceAttr("xyops_category.test", "notes", "Managed by Terraform"),
					resource.TestCheckResourceAttr("xyops_category.test", "actions", "[]"),
					resource.TestCheckResourceAttr("xyops_category.test", "limits", "[]"),
				),
			},
			{
				ResourceName:            "xyops_category.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"actions", "limits"},
			},
			{
				Config: providerConfig() + `
resource "xyops_category" "test" {
  id      = "tfacccategory"
  title   = "TF Acc Category Updated"
  enabled = true
  icon    = "folder"
  color   = "blue"
  notes   = "Managed by Terraform"
  actions = "[]"
  limits  = "[]"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_category.test", "id", "tfacccategory"),
					resource.TestCheckResourceAttr("xyops_category.test", "title", "TF Acc Category Updated"),
				),
			},
		},
	})
}

func TestAccCategoryResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_category" "test" {
  id      = "tfacccategory"
  title   = "TF Acc Category Drift"
  enabled = true
  icon    = "folder"
  color   = "blue"
  notes   = "Managed by Terraform"
  actions = "[]"
  limits  = "[]"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_category.test", "id", "tfacccategory"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_category", map[string]string{"id": testAccCategoryID()}, nil)
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
					resource.TestCheckResourceAttr("xyops_category.test", "id", "tfacccategory"),
					resource.TestCheckResourceAttr("xyops_category.test", "title", "TF Acc Category Drift"),
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

func TestAccCategoryResource_omittedJSON(t *testing.T) {
	config := providerConfig() + `
resource "xyops_category" "test" {
  id    = "tfacccategory"
  title = "TF Acc Category Omitted"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_category.test", "id", "tfacccategory"),
					resource.TestCheckNoResourceAttr("xyops_category.test", "actions"),
					resource.TestCheckNoResourceAttr("xyops_category.test", "limits"),
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccCategoryID() string { return "tfacccategory" }
