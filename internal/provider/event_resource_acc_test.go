package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccEventResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_event" "test" {
  id       = "tfaccevent"
  title    = "TF Acc Event"
  enabled  = true
  category = "general"
  plugin   = "shellplug"
  targets  = ["local"]
  params   = jsonencode({ script = "#!/bin/sh\necho hello" })
  algo     = "random"
  notes    = "Managed by Terraform"
  fields   = "[]"
  tags     = []
  triggers = "[]"
  limits   = "[]"
  actions  = "[]"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_event.test", "id", "tfaccevent"),
					resource.TestCheckResourceAttr("xyops_event.test", "title", "TF Acc Event"),
					resource.TestCheckResourceAttr("xyops_event.test", "enabled", "true"),
					resource.TestCheckResourceAttr("xyops_event.test", "category", "general"),
					resource.TestCheckResourceAttr("xyops_event.test", "plugin", "shellplug"),
					resource.TestCheckResourceAttr("xyops_event.test", "targets.#", "1"),
					resource.TestCheckResourceAttr("xyops_event.test", "targets.0", "local"),
					resource.TestCheckResourceAttr("xyops_event.test", "algo", "random"),
					resource.TestCheckResourceAttr("xyops_event.test", "notes", "Managed by Terraform"),
					resource.TestCheckResourceAttr("xyops_event.test", "params", `{"script":"#!/bin/sh\necho hello"}`),
				),
			},
			{
				Config: providerConfig() + `
resource "xyops_event" "test" {
  id       = "tfaccevent"
  title    = "TF Acc Event Updated"
  enabled  = true
  category = "general"
  plugin   = "shellplug"
  targets  = ["local"]
  params   = jsonencode({ script = "#!/bin/sh\necho hello" })
  algo     = "random"
  notes    = "Managed by Terraform"
  fields   = "[]"
  tags     = []
  triggers = "[]"
  limits   = "[]"
  actions  = "[]"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_event.test", "id", "tfaccevent"),
					resource.TestCheckResourceAttr("xyops_event.test", "title", "TF Acc Event Updated"),
				),
			},
			{
				ResourceName:            "xyops_event.test",
				ImportState:             true,
				ImportStateId:           "tfaccevent",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"params", "fields", "triggers", "limits", "actions"},
			},
		},
	})
}

func TestAccEventResource_targetsTagsOrder(t *testing.T) {
	unsortedConfig := providerConfig() + `
resource "xyops_tag" "zeta" {
  id    = "tfacceventtagz"
  title = "TF Acc Event Tag Z"
}

resource "xyops_tag" "alpha" {
  id    = "tfacceventtaga"
  title = "TF Acc Event Tag A"
}

resource "xyops_event" "test" {
  id       = "tfaccevent"
  title    = "TF Acc Event Order"
  enabled  = true
  category = "general"
  plugin   = "shellplug"
  targets  = ["zeta", "alpha"]
  params   = jsonencode({ script = "#!/bin/sh\necho order" })
  algo     = "random"
  tags     = [xyops_tag.zeta.id, xyops_tag.alpha.id]
  fields   = "[]"
  triggers = "[]"
  limits   = "[]"
  actions  = "[]"
}
`
	sortedConfig := providerConfig() + `
resource "xyops_tag" "zeta" {
  id    = "tfacceventtagz"
  title = "TF Acc Event Tag Z"
}

resource "xyops_tag" "alpha" {
  id    = "tfacceventtaga"
  title = "TF Acc Event Tag A"
}

resource "xyops_event" "test" {
  id       = "tfaccevent"
  title    = "TF Acc Event Order"
  enabled  = true
  category = "general"
  plugin   = "shellplug"
  targets  = ["alpha", "zeta"]
  params   = jsonencode({ script = "#!/bin/sh\necho order" })
  algo     = "random"
  tags     = [xyops_tag.alpha.id, xyops_tag.zeta.id]
  fields   = "[]"
  triggers = "[]"
  limits   = "[]"
  actions  = "[]"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_event.test", "id", "tfaccevent"),
					resource.TestCheckResourceAttr("xyops_event.test", "targets.#", "2"),
					resource.TestCheckResourceAttr("xyops_event.test", "targets.0", "zeta"),
					resource.TestCheckResourceAttr("xyops_event.test", "targets.1", "alpha"),
					resource.TestCheckResourceAttr("xyops_event.test", "tags.#", "2"),
					resource.TestCheckResourceAttr("xyops_event.test", "tags.0", "tfacceventtagz"),
					resource.TestCheckResourceAttr("xyops_event.test", "tags.1", "tfacceventtaga"),
				),
			},
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_event.test", "targets.0", "zeta"),
					resource.TestCheckResourceAttr("xyops_event.test", "tags.0", "tfacceventtagz"),
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
					resource.TestCheckResourceAttr("xyops_event.test", "targets.0", "alpha"),
					resource.TestCheckResourceAttr("xyops_event.test", "targets.1", "zeta"),
					resource.TestCheckResourceAttr("xyops_event.test", "tags.0", "tfacceventtaga"),
					resource.TestCheckResourceAttr("xyops_event.test", "tags.1", "tfacceventtagz"),
				),
			},
			{
				Config: sortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_event.test", "targets.0", "alpha"),
					resource.TestCheckResourceAttr("xyops_event.test", "tags.0", "tfacceventtaga"),
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

func TestAccEventResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_event" "test" {
  id       = "tfaccevent"
  title    = "TF Acc Event Drift"
  enabled  = true
  category = "general"
  plugin   = "shellplug"
  targets  = ["local"]
  params   = jsonencode({ script = "#!/bin/sh\necho hello" })
  algo     = "random"
  notes    = "Managed by Terraform"
  fields   = "[]"
  tags     = []
  triggers = "[]"
  limits   = "[]"
  actions  = "[]"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_event.test", "id", "tfaccevent"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_event", map[string]string{"id": testAccEventID()}, nil)
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
					resource.TestCheckResourceAttr("xyops_event.test", "id", "tfaccevent"),
					resource.TestCheckResourceAttr("xyops_event.test", "title", "TF Acc Event Drift"),
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
