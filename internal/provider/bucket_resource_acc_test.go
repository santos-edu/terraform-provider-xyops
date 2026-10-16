package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccBucketResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_bucket" "test" {
  id    = "tfaccbucket"
  title = "TF Acc Bucket"
  data  = jsonencode({ environment = "dev" })
  notes = "Managed by Terraform"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_bucket.test", "id", "tfaccbucket"),
					resource.TestCheckResourceAttr("xyops_bucket.test", "title", "TF Acc Bucket"),
					resource.TestCheckResourceAttr("xyops_bucket.test", "enabled", "true"),
					resource.TestCheckResourceAttr("xyops_bucket.test", "notes", "Managed by Terraform"),
					resource.TestCheckResourceAttr("xyops_bucket.test", "data", `{"environment":"dev"}`),
				),
			},
			{
				Config: providerConfig() + `
resource "xyops_bucket" "test" {
  id    = "tfaccbucket"
  title = "TF Acc Bucket Updated"
  data  = jsonencode({ environment = "prod" })
  notes = "Managed by Terraform"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_bucket.test", "id", "tfaccbucket"),
					resource.TestCheckResourceAttr("xyops_bucket.test", "title", "TF Acc Bucket Updated"),
					resource.TestCheckResourceAttr("xyops_bucket.test", "data", `{"environment":"prod"}`),
				),
			},
			{
				ResourceName:            "xyops_bucket.test",
				ImportState:             true,
				ImportStateId:           "tfaccbucket",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"data"},
			},
		},
	})
}

func TestAccBucketResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_bucket" "test" {
  id    = "tfaccbucket"
  title = "TF Acc Bucket Drift"
  data  = jsonencode({ environment = "dev" })
  notes = "Managed by Terraform"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_bucket.test", "id", "tfaccbucket"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_bucket", map[string]string{"id": testAccBucketID()}, nil)
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
					resource.TestCheckResourceAttr("xyops_bucket.test", "id", "tfaccbucket"),
					resource.TestCheckResourceAttr("xyops_bucket.test", "title", "TF Acc Bucket Drift"),
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
