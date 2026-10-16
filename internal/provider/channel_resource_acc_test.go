package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccChannelResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_channel" "test" {
  id        = "tfaccchannel"
  title     = "TF Acc Channel"
  enabled   = true
  notes     = "Managed by Terraform"
  users     = ["admin"]
  email     = "ops@example.com"
  web_hook  = ""
  run_event = ""
  sound     = "attention-3.mp3"
  icon      = "bell"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_channel.test", "id", "tfaccchannel"),
					resource.TestCheckResourceAttr("xyops_channel.test", "title", "TF Acc Channel"),
					resource.TestCheckResourceAttr("xyops_channel.test", "enabled", "true"),
					resource.TestCheckResourceAttr("xyops_channel.test", "notes", "Managed by Terraform"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.#", "1"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.0", "admin"),
					resource.TestCheckResourceAttr("xyops_channel.test", "email", "ops@example.com"),
					resource.TestCheckResourceAttr("xyops_channel.test", "web_hook", ""),
					resource.TestCheckResourceAttr("xyops_channel.test", "run_event", ""),
					resource.TestCheckResourceAttr("xyops_channel.test", "sound", "attention-3.mp3"),
					resource.TestCheckResourceAttr("xyops_channel.test", "icon", "bell"),
				),
			},
			{
				Config: providerConfig() + `
resource "xyops_channel" "test" {
  id        = "tfaccchannel"
  title     = "TF Acc Channel Updated"
  enabled   = false
  notes     = "Managed by Terraform v2"
  users     = []
  email     = "sre@example.com"
  web_hook  = "https://example.com/hook"
  run_event = "1"
  sound     = "attention-1.mp3"
  icon      = "bell"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_channel.test", "id", "tfaccchannel"),
					resource.TestCheckResourceAttr("xyops_channel.test", "title", "TF Acc Channel Updated"),
					resource.TestCheckResourceAttr("xyops_channel.test", "enabled", "false"),
					resource.TestCheckResourceAttr("xyops_channel.test", "notes", "Managed by Terraform v2"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.#", "0"),
					resource.TestCheckResourceAttr("xyops_channel.test", "email", "sre@example.com"),
					resource.TestCheckResourceAttr("xyops_channel.test", "web_hook", "https://example.com/hook"),
					resource.TestCheckResourceAttr("xyops_channel.test", "run_event", "1"),
					resource.TestCheckResourceAttr("xyops_channel.test", "sound", "attention-1.mp3"),
				),
			},
			{
				ResourceName:      "xyops_channel.test",
				ImportState:       true,
				ImportStateId:     "tfaccchannel",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccChannelResource_omittedOptionals(t *testing.T) {
	updatedConfig := providerConfig() + `
resource "xyops_channel" "test" {
  id    = "tfaccchannel"
  title = "TF Acc Channel Omitted Optionals Updated"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_channel" "test" {
  id    = "tfaccchannel"
  title = "TF Acc Channel Omitted Optionals"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_channel.test", "id", "tfaccchannel"),
					resource.TestCheckResourceAttr("xyops_channel.test", "title", "TF Acc Channel Omitted Optionals"),
					resource.TestCheckResourceAttr("xyops_channel.test", "enabled", "true"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.#", "0"),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_channel.test", "id", "tfaccchannel"),
					resource.TestCheckResourceAttr("xyops_channel.test", "title", "TF Acc Channel Omitted Optionals Updated"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.#", "0"),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_channel.test", "id", "tfaccchannel"),
					resource.TestCheckResourceAttr("xyops_channel.test", "title", "TF Acc Channel Omitted Optionals Updated"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.#", "0"),
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

func TestAccChannelResource_usersOrder(t *testing.T) {
	unsortedConfig := providerConfig() + `
resource "xyops_channel" "test" {
  id    = "tfaccchannel"
  title = "TF Acc Channel Users Order"
  users = ["zeta", "alpha"]
}
`
	sortedConfig := providerConfig() + `
resource "xyops_channel" "test" {
  id    = "tfaccchannel"
  title = "TF Acc Channel Users Order"
  users = ["alpha", "zeta"]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_channel.test", "id", "tfaccchannel"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.#", "2"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.0", "zeta"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.1", "alpha"),
				),
			},
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_channel.test", "users.#", "2"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.0", "zeta"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.1", "alpha"),
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
					resource.TestCheckResourceAttr("xyops_channel.test", "users.#", "2"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.0", "alpha"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.1", "zeta"),
				),
			},
			{
				Config: sortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_channel.test", "users.#", "2"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.0", "alpha"),
					resource.TestCheckResourceAttr("xyops_channel.test", "users.1", "zeta"),
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

func TestAccChannelResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_channel" "test" {
  id        = "tfaccchannel"
  title     = "TF Acc Channel Drift"
  enabled   = true
  notes     = "Managed by Terraform"
  users     = ["admin"]
  email     = "ops@example.com"
  web_hook  = ""
  run_event = ""
  sound     = "attention-3.mp3"
  icon      = "bell"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_channel.test", "id", "tfaccchannel"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_channel", map[string]string{"id": "tfaccchannel"}, nil)
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
					resource.TestCheckResourceAttr("xyops_channel.test", "id", "tfaccchannel"),
					resource.TestCheckResourceAttr("xyops_channel.test", "title", "TF Acc Channel Drift"),
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
