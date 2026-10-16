package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccPluginResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_plugin" "test" {
  id          = "tfaccplugin"
  title       = "TF Acc Plugin"
  type        = "event"
  command     = "/bin/sh"
  script_file = "#!/bin/sh\necho hello"
  uid         = 0
  gid         = 0
  params      = jsonencode([{ id = "foo", type = "text", title = "Foo", value = "bar" }])
  groups      = []
  format      = ""
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_plugin.test", "id", "tfaccplugin"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "title", "TF Acc Plugin"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "type", "event"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "command", "/bin/sh"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "script_file", "#!/bin/sh\necho hello"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "uid", "0"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "gid", "0"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "params", `[{"id":"foo","title":"Foo","type":"text","value":"bar"}]`),
				),
			},
			{
				Config: providerConfig() + `
resource "xyops_plugin" "test" {
  id          = "tfaccplugin"
  title       = "TF Acc Plugin Updated"
  type        = "event"
  command     = "/bin/bash"
  script_file = "#!/bin/bash\necho world"
  uid         = 0
  gid         = 0
  params      = jsonencode([{ id = "foo", type = "text", title = "Foo", value = "baz" }])
  groups      = []
  format      = ""
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_plugin.test", "id", "tfaccplugin"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "title", "TF Acc Plugin Updated"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "command", "/bin/bash"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "script_file", "#!/bin/bash\necho world"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "params", `[{"id":"foo","title":"Foo","type":"text","value":"baz"}]`),
				),
			},
			{
				ResourceName:            "xyops_plugin.test",
				ImportState:             true,
				ImportStateId:           "tfaccplugin",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"format", "uid", "gid", "params"},
			},
		},
	})
}

func TestAccPluginResource_groupsOrder(t *testing.T) {
	unsortedConfig := providerConfig() + `
resource "xyops_plugin" "test" {
  id          = "tfaccplugin"
  title       = "TF Acc Plugin Groups Order"
  type        = "event"
  command     = "/bin/sh"
  script_file = "#!/bin/sh\necho order"
  groups      = ["zeta", "alpha"]
}
`
	sortedConfig := providerConfig() + `
resource "xyops_plugin" "test" {
  id          = "tfaccplugin"
  title       = "TF Acc Plugin Groups Order"
  type        = "event"
  command     = "/bin/sh"
  script_file = "#!/bin/sh\necho order"
  groups      = ["alpha", "zeta"]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_plugin.test", "id", "tfaccplugin"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "groups.#", "2"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "groups.0", "zeta"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "groups.1", "alpha"),
				),
			},
			{
				Config: unsortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_plugin.test", "groups.0", "zeta"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "groups.1", "alpha"),
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
					resource.TestCheckResourceAttr("xyops_plugin.test", "groups.0", "alpha"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "groups.1", "zeta"),
				),
			},
			{
				Config: sortedConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_plugin.test", "groups.0", "alpha"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "groups.1", "zeta"),
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

func TestAccPluginResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_plugin" "test" {
  id          = "tfaccplugin"
  title       = "TF Acc Plugin Drift"
  type        = "event"
  command     = "/bin/sh"
  script_file = "#!/bin/sh\necho drift"
  uid         = 0
  gid         = 0
  params      = jsonencode([])
  groups      = []
  format      = ""
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_plugin.test", "id", "tfaccplugin"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_plugin", map[string]string{"id": "tfaccplugin"}, nil)
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
					resource.TestCheckResourceAttr("xyops_plugin.test", "id", "tfaccplugin"),
					resource.TestCheckResourceAttr("xyops_plugin.test", "title", "TF Acc Plugin Drift"),
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
