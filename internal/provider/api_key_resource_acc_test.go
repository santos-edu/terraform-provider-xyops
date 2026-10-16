package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccAPIKeyResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_api_key" "test" {
  id          = "tfaccapikey"
  title       = "TF Acc API Key"
  description = "Managed by Terraform"
  active      = true
  privileges  = { admin = 1 }
  roles       = []
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_api_key.test", "id", "tfaccapikey"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "title", "TF Acc API Key"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "description", "Managed by Terraform"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "active", "true"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "privileges.admin", "1"),
					resource.TestCheckResourceAttrSet("xyops_api_key.test", "key"),
					resource.TestCheckResourceAttrSet("xyops_api_key.test", "plain_key"),
				),
			},
			{
				Config: providerConfig() + `
resource "xyops_api_key" "test" {
  id          = "tfaccapikey"
  title       = "TF Acc API Key Updated"
  description = "Managed by Terraform"
  active      = true
  privileges  = { admin = 1 }
  roles       = []
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_api_key.test", "id", "tfaccapikey"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "title", "TF Acc API Key Updated"),
				),
			},
			{
				ResourceName:            "xyops_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key", "plain_key"},
			},
		},
	})
}

func TestAccAPIKeyResource_rolesOrder(t *testing.T) {
	unsortedConfig := providerConfig() + `
resource "xyops_role" "z" {
  id         = "tfacckeyrolez"
  title      = "TF Acc Key Role Z"
  privileges = { admin = 1 }
  categories = []
}

resource "xyops_role" "a" {
  id         = "tfacckeyrolea"
  title      = "TF Acc Key Role A"
  privileges = { admin = 1 }
  categories = []
}

resource "xyops_api_key" "test" {
  id          = "tfaccapikey"
  title       = "TF Acc API Key Roles Order"
  description = "Managed by Terraform"
  active      = true
  privileges  = { admin = 1 }
  roles       = [xyops_role.z.id, xyops_role.a.id]
}
`
	sortedConfig := providerConfig() + `
resource "xyops_role" "z" {
  id         = "tfacckeyrolez"
  title      = "TF Acc Key Role Z"
  privileges = { admin = 1 }
  categories = []
}

resource "xyops_role" "a" {
  id         = "tfacckeyrolea"
  title      = "TF Acc Key Role A"
  privileges = { admin = 1 }
  categories = []
}

resource "xyops_api_key" "test" {
  id          = "tfaccapikey"
  title       = "TF Acc API Key Roles Order"
  description = "Managed by Terraform"
  active      = true
  privileges  = { admin = 1 }
  roles       = [xyops_role.a.id, xyops_role.z.id]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.#", "2"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.0", "tfacckeyrolez"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.1", "tfacckeyrolea"),
				),
			},
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.#", "2"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.0", "tfacckeyrolez"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.1", "tfacckeyrolea"),
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
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.#", "2"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.0", "tfacckeyrolea"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.1", "tfacckeyrolez"),
				),
			},
			{
				Config: sortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.#", "2"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.0", "tfacckeyrolea"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.1", "tfacckeyrolez"),
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

func TestAccAPIKeyResource_omittedComputed(t *testing.T) {
	config := providerConfig() + `
resource "xyops_api_key" "test" {
  id         = "tfaccapikey"
  title      = "TF Acc API Key Omitted"
  privileges = { admin = 1 }
}
`
	updatedConfig := providerConfig() + `
resource "xyops_api_key" "test" {
  id         = "tfaccapikey"
  title      = "TF Acc API Key Omitted Updated"
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
					resource.TestCheckResourceAttr("xyops_api_key.test", "id", "tfaccapikey"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.#", "0"),
					resource.TestCheckResourceAttrSet("xyops_api_key.test", "key"),
					resource.TestCheckResourceAttrSet("xyops_api_key.test", "plain_key"),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_api_key.test", "title", "TF Acc API Key Omitted Updated"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.#", "0"),
					resource.TestCheckResourceAttrSet("xyops_api_key.test", "key"),
					resource.TestCheckResourceAttrSet("xyops_api_key.test", "plain_key"),
				),
			},
			{
				ResourceName:            "xyops_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key", "plain_key"},
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_api_key.test", "roles.#", "0"),
					resource.TestCheckResourceAttrSet("xyops_api_key.test", "key"),
					resource.TestCheckResourceAttrSet("xyops_api_key.test", "plain_key"),
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

func TestAccAPIKeyResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_api_key" "test" {
  id          = "tfaccapikey"
  title       = "TF Acc API Key Drift"
  description = "Managed by Terraform"
  active      = true
  privileges  = { admin = 1 }
  roles       = []
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_api_key.test", "id", "tfaccapikey"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_api_key", map[string]string{"id": testAccAPIKeyID()}, nil)
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
					resource.TestCheckResourceAttr("xyops_api_key.test", "id", "tfaccapikey"),
					resource.TestCheckResourceAttr("xyops_api_key.test", "title", "TF Acc API Key Drift"),
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
