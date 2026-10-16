package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccSecretResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_secret" "test" {
  id       = "tfaccsecret"
  title    = "TF Acc Secret"
  enabled  = true
  notes    = "Managed by Terraform"
  value    = "s3cret"
  category = []
  params   = jsonencode([])
  icon     = "key"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_secret.test", "id", "tfaccsecret"),
					resource.TestCheckResourceAttr("xyops_secret.test", "title", "TF Acc Secret"),
					resource.TestCheckResourceAttr("xyops_secret.test", "enabled", "true"),
					resource.TestCheckResourceAttr("xyops_secret.test", "notes", "Managed by Terraform"),
					resource.TestCheckResourceAttr("xyops_secret.test", "value", "s3cret"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.#", "0"),
					resource.TestCheckResourceAttr("xyops_secret.test", "params", "[]"),
					resource.TestCheckResourceAttr("xyops_secret.test", "icon", "key"),
				),
			},
			{
				Config: providerConfig() + `
resource "xyops_secret" "test" {
  id       = "tfaccsecret"
  title    = "TF Acc Secret Updated"
  enabled  = false
  notes    = "Managed by Terraform v2"
  value    = "s3cret-v2"
  category = []
  params   = jsonencode([{ id = "user", title = "User", type = "text", value = "admin" }])
  icon     = "key"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_secret.test", "id", "tfaccsecret"),
					resource.TestCheckResourceAttr("xyops_secret.test", "title", "TF Acc Secret Updated"),
					resource.TestCheckResourceAttr("xyops_secret.test", "enabled", "false"),
					resource.TestCheckResourceAttr("xyops_secret.test", "notes", "Managed by Terraform v2"),
					resource.TestCheckResourceAttr("xyops_secret.test", "value", "s3cret-v2"),
					resource.TestCheckResourceAttr("xyops_secret.test", "params", `[{"id":"user","title":"User","type":"text","value":"admin"}]`),
				),
			},
			{
				ResourceName:            "xyops_secret.test",
				ImportState:             true,
				ImportStateId:           "tfaccsecret",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value"},
			},
		},
	})
}

func TestAccSecretResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_secret" "test" {
  id       = "tfaccsecret"
  title    = "TF Acc Secret Drift"
  enabled  = true
  notes    = "Managed by Terraform"
  value    = "s3cret"
  category = []
  params   = jsonencode([])
  icon     = "key"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_secret.test", "id", "tfaccsecret"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_secret", map[string]string{"id": "tfaccsecret"}, nil)
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
					resource.TestCheckResourceAttr("xyops_secret.test", "id", "tfaccsecret"),
					resource.TestCheckResourceAttr("xyops_secret.test", "title", "TF Acc Secret Drift"),
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

func TestAccSecretResource_categoryOrder(t *testing.T) {
	unsortedConfig := providerConfig() + `
resource "xyops_secret" "test" {
  id       = "tfaccsecret"
  title    = "TF Acc Secret Category Order"
  value    = "s3cret"
  category = ["zeta", "alpha"]
}
`
	sortedConfig := providerConfig() + `
resource "xyops_secret" "test" {
  id       = "tfaccsecret"
  title    = "TF Acc Secret Category Order"
  value    = "s3cret"
  category = ["alpha", "zeta"]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_secret.test", "id", "tfaccsecret"),
					resource.TestCheckResourceAttr("xyops_secret.test", "params", "[]"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.#", "2"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.0", "zeta"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.1", "alpha"),
				),
			},
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_secret.test", "category.#", "2"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.0", "zeta"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.1", "alpha"),
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
					resource.TestCheckResourceAttr("xyops_secret.test", "category.#", "2"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.0", "alpha"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.1", "zeta"),
				),
			},
			{
				Config: sortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_secret.test", "category.#", "2"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.0", "alpha"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.1", "zeta"),
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

func TestAccSecretResource_omittedOptionals(t *testing.T) {
	updatedConfig := providerConfig() + `
resource "xyops_secret" "test" {
  id    = "tfaccsecret"
  title = "TF Acc Secret Omitted Optionals Updated"
  value = "s3cret"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_secret" "test" {
  id    = "tfaccsecret"
  title = "TF Acc Secret Omitted Optionals"
  value = "s3cret"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_secret.test", "id", "tfaccsecret"),
					resource.TestCheckResourceAttr("xyops_secret.test", "title", "TF Acc Secret Omitted Optionals"),
					resource.TestCheckResourceAttr("xyops_secret.test", "enabled", "true"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.#", "0"),
					resource.TestCheckResourceAttr("xyops_secret.test", "params", "[]"),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_secret.test", "id", "tfaccsecret"),
					resource.TestCheckResourceAttr("xyops_secret.test", "title", "TF Acc Secret Omitted Optionals Updated"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.#", "0"),
					resource.TestCheckResourceAttr("xyops_secret.test", "params", "[]"),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_secret.test", "id", "tfaccsecret"),
					resource.TestCheckResourceAttr("xyops_secret.test", "title", "TF Acc Secret Omitted Optionals Updated"),
					resource.TestCheckResourceAttr("xyops_secret.test", "category.#", "0"),
					resource.TestCheckResourceAttr("xyops_secret.test", "params", "[]"),
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

func TestAccSecretResource_omittedParams(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_secret" "test" {
  id    = "tfaccsecret"
  title = "TF Acc Secret Omitted Params"
  notes = "Managed by Terraform"
  value = "s3cret"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_secret.test", "id", "tfaccsecret"),
					resource.TestCheckResourceAttr("xyops_secret.test", "title", "TF Acc Secret Omitted Params"),
					resource.TestCheckResourceAttr("xyops_secret.test", "notes", "Managed by Terraform"),
					resource.TestCheckResourceAttr("xyops_secret.test", "value", "s3cret"),
					resource.TestCheckResourceAttr("xyops_secret.test", "params", "[]"),
				),
			},
			{
				ResourceName:            "xyops_secret.test",
				ImportState:             true,
				ImportStateId:           "tfaccsecret",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value"},
			},
		},
	})
}
