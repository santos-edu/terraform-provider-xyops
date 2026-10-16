package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccWorkflowResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_workflow" "test" {
  id       = "tfaccworkflow"
  title    = "TF Acc Workflow"
  enabled  = true
  category = "general"
  tags     = []
  triggers = "[]"
  limits   = "[]"
  actions  = "[]"
  workflow = jsonencode({
    nodes = [
      { id = "start", type = "event", event = "shellplug", x = 10, y = 10 }
    ]
    connections = []
  })
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_workflow.test", "id", "tfaccworkflow"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "title", "TF Acc Workflow"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "enabled", "true"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "category", "general"),
				),
			},
			{
				Config: providerConfig() + `
resource "xyops_workflow" "test" {
  id       = "tfaccworkflow"
  title    = "TF Acc Workflow Updated"
  enabled  = true
  category = "general"
  tags     = []
  triggers = "[]"
  limits   = "[]"
  actions  = "[]"
  workflow = jsonencode({
    nodes = [
      { id = "start", type = "event", event = "shellplug", x = 10, y = 10 }
    ]
    connections = []
  })
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_workflow.test", "id", "tfaccworkflow"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "title", "TF Acc Workflow Updated"),
				),
			},
			{
				ResourceName:            "xyops_workflow.test",
				ImportState:             true,
				ImportStateId:           "tfaccworkflow",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"triggers", "limits", "actions"},
			},
		},
	})
}

func TestAccWorkflowResource_tagsOrder(t *testing.T) {
	unsortedConfig := providerConfig() + `
resource "xyops_tag" "z" {
  id      = "tfacctagzzz"
  title   = "TF Acc Tag Z"
  enabled = true
}

resource "xyops_tag" "a" {
  id      = "tfacctagaaa"
  title   = "TF Acc Tag A"
  enabled = true
}

resource "xyops_workflow" "test" {
  id       = "tfaccworkflow"
  title    = "TF Acc Workflow Tags Order"
  enabled  = true
  category = "general"
  tags     = [xyops_tag.z.id, xyops_tag.a.id]
  triggers = "[]"
  limits   = "[]"
  actions  = "[]"
  workflow = jsonencode({
    nodes = [
      { id = "start", type = "event", event = "shellplug", x = 10, y = 10 }
    ]
    connections = []
  })
}
`
	sortedConfig := providerConfig() + `
resource "xyops_tag" "z" {
  id      = "tfacctagzzz"
  title   = "TF Acc Tag Z"
  enabled = true
}

resource "xyops_tag" "a" {
  id      = "tfacctagaaa"
  title   = "TF Acc Tag A"
  enabled = true
}

resource "xyops_workflow" "test" {
  id       = "tfaccworkflow"
  title    = "TF Acc Workflow Tags Order"
  enabled  = true
  category = "general"
  tags     = [xyops_tag.a.id, xyops_tag.z.id]
  triggers = "[]"
  limits   = "[]"
  actions  = "[]"
  workflow = jsonencode({
    nodes = [
      { id = "start", type = "event", event = "shellplug", x = 10, y = 10 }
    ]
    connections = []
  })
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.#", "2"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.0", "tfacctagzzz"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.1", "tfacctagaaa"),
				),
			},
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.#", "2"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.0", "tfacctagzzz"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.1", "tfacctagaaa"),
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
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.#", "2"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.0", "tfacctagaaa"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.1", "tfacctagzzz"),
				),
			},
			{
				Config: sortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.#", "2"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.0", "tfacctagaaa"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.1", "tfacctagzzz"),
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

func TestAccWorkflowResource_omittedComputed(t *testing.T) {
	config := providerConfig() + `
resource "xyops_workflow" "test" {
  id       = "tfaccworkflow"
  title    = "TF Acc Workflow Omitted"
  category = "general"
  triggers = "[]"
  limits   = "[]"
  actions  = "[]"
  workflow = jsonencode({
    nodes = [
      { id = "start", type = "event", event = "shellplug", x = 10, y = 10 }
    ]
    connections = []
  })
}
`
	updatedConfig := providerConfig() + `
resource "xyops_workflow" "test" {
  id       = "tfaccworkflow"
  title    = "TF Acc Workflow Omitted Updated"
  category = "general"
  triggers = "[]"
  limits   = "[]"
  actions  = "[]"
  workflow = jsonencode({
    nodes = [
      { id = "start", type = "event", event = "shellplug", x = 10, y = 10 }
    ]
    connections = []
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
					resource.TestCheckResourceAttr("xyops_workflow.test", "id", "tfaccworkflow"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.#", "0"),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_workflow.test", "title", "TF Acc Workflow Omitted Updated"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.#", "0"),
				),
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_workflow.test", "tags.#", "0"),
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

func TestAccWorkflowResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_workflow" "test" {
  id       = "tfaccworkflow"
  title    = "TF Acc Workflow Drift"
  enabled  = true
  category = "general"
  tags     = []
  triggers = "[]"
  limits   = "[]"
  actions  = "[]"
  workflow = jsonencode({
    nodes = [
      { id = "start", type = "event", event = "shellplug", x = 10, y = 10 }
    ]
    connections = []
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
					resource.TestCheckResourceAttr("xyops_workflow.test", "id", "tfaccworkflow"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_event", map[string]string{"id": testAccWorkflowID()}, nil)
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
					resource.TestCheckResourceAttr("xyops_workflow.test", "id", "tfaccworkflow"),
					resource.TestCheckResourceAttr("xyops_workflow.test", "title", "TF Acc Workflow Drift"),
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
