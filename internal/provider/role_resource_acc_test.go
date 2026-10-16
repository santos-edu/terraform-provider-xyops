package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccRoleResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_role" "test" {
  id         = "tfaccrole"
  title      = "TF Acc Role"
  enabled    = true
  privileges = { admin = 1 }
  categories = []
  notes      = "Managed by Terraform"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_role.test", "id", "tfaccrole"),
					resource.TestCheckResourceAttr("xyops_role.test", "title", "TF Acc Role"),
					resource.TestCheckResourceAttr("xyops_role.test", "enabled", "true"),
					resource.TestCheckResourceAttr("xyops_role.test", "privileges.admin", "1"),
					resource.TestCheckResourceAttr("xyops_role.test", "notes", "Managed by Terraform"),
				),
			},
			{
				Config: providerConfig() + `
resource "xyops_role" "test" {
  id         = "tfaccrole"
  title      = "TF Acc Role Updated"
  enabled    = true
  privileges = { admin = 1 }
  categories = []
  notes      = "Managed by Terraform"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_role.test", "id", "tfaccrole"),
					resource.TestCheckResourceAttr("xyops_role.test", "title", "TF Acc Role Updated"),
				),
			},
			{
				ResourceName:      "xyops_role.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccRoleResource_categoriesOrder(t *testing.T) {
	unsortedConfig := providerConfig() + `
resource "xyops_category" "z" {
  id      = "tfacccatzzz"
  title   = "TF Acc Cat Z"
  enabled = true
  actions = "[]"
  limits  = "[]"
}

resource "xyops_category" "a" {
  id      = "tfacccataaa"
  title   = "TF Acc Cat A"
  enabled = true
  actions = "[]"
  limits  = "[]"
}

resource "xyops_role" "test" {
  id         = "tfaccrole"
  title      = "TF Acc Role Categories Order"
  privileges = { admin = 1 }
  categories = [xyops_category.z.id, xyops_category.a.id]
}
`
	sortedConfig := providerConfig() + `
resource "xyops_category" "z" {
  id      = "tfacccatzzz"
  title   = "TF Acc Cat Z"
  enabled = true
  actions = "[]"
  limits  = "[]"
}

resource "xyops_category" "a" {
  id      = "tfacccataaa"
  title   = "TF Acc Cat A"
  enabled = true
  actions = "[]"
  limits  = "[]"
}

resource "xyops_role" "test" {
  id         = "tfaccrole"
  title      = "TF Acc Role Categories Order"
  privileges = { admin = 1 }
  categories = [xyops_category.a.id, xyops_category.z.id]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_role.test", "categories.#", "2"),
					resource.TestCheckResourceAttr("xyops_role.test", "categories.0", "tfacccatzzz"),
					resource.TestCheckResourceAttr("xyops_role.test", "categories.1", "tfacccataaa"),
				),
			},
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_role.test", "categories.#", "2"),
					resource.TestCheckResourceAttr("xyops_role.test", "categories.0", "tfacccatzzz"),
					resource.TestCheckResourceAttr("xyops_role.test", "categories.1", "tfacccataaa"),
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
					resource.TestCheckResourceAttr("xyops_role.test", "categories.#", "2"),
					resource.TestCheckResourceAttr("xyops_role.test", "categories.0", "tfacccataaa"),
					resource.TestCheckResourceAttr("xyops_role.test", "categories.1", "tfacccatzzz"),
				),
			},
			{
				Config: sortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_role.test", "categories.#", "2"),
					resource.TestCheckResourceAttr("xyops_role.test", "categories.0", "tfacccataaa"),
					resource.TestCheckResourceAttr("xyops_role.test", "categories.1", "tfacccatzzz"),
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

func TestAccRoleResource_omittedComputed(t *testing.T) {
	config := providerConfig() + `
resource "xyops_role" "test" {
  id         = "tfaccrole"
  title      = "TF Acc Role Omitted"
  privileges = { admin = 1 }
}
`
	updatedConfig := providerConfig() + `
resource "xyops_role" "test" {
  id         = "tfaccrole"
  title      = "TF Acc Role Omitted Updated"
  privileges = { admin = 1 }
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_role.test", "id", "tfaccrole"),
					resource.TestCheckResourceAttr("xyops_role.test", "categories.#", "0"),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_role.test", "title", "TF Acc Role Omitted Updated"),
					resource.TestCheckResourceAttr("xyops_role.test", "categories.#", "0"),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_role.test", "categories.#", "0"),
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

func TestAccRoleResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_role" "test" {
  id         = "tfaccrole"
  title      = "TF Acc Role Drift"
  enabled    = true
  privileges = { admin = 1 }
  categories = []
  notes      = "Managed by Terraform"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_role.test", "id", "tfaccrole"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_role", map[string]string{"id": testAccRoleID()}, nil)
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
					resource.TestCheckResourceAttr("xyops_role.test", "id", "tfaccrole"),
					resource.TestCheckResourceAttr("xyops_role.test", "title", "TF Acc Role Drift"),
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
