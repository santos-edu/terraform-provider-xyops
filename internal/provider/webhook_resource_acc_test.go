package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccWebhookResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "xyops_webhook" "test" {
  id              = "tfaccwebhook"
  title           = "TF Acc Webhook"
  url             = "https://httpbin.org/post"
  method          = "POST"
  headers         = jsonencode([{ name = "Content-Type", value = "application/json" }])
  body            = "{}"
  timeout         = 30
  retries         = 0
  follow          = false
  ssl_cert_bypass = false
  max_per_day     = 0
  notes           = "Managed by Terraform"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_webhook.test", "id", "tfaccwebhook"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "title", "TF Acc Webhook"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "url", "https://httpbin.org/post"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "method", "POST"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "headers", `[{"name":"Content-Type","value":"application/json"}]`),
					resource.TestCheckResourceAttr("xyops_webhook.test", "body", "{}"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "timeout", "30"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "retries", "0"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "follow", "false"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "ssl_cert_bypass", "false"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "max_per_day", "0"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "notes", "Managed by Terraform"),
				),
			},
			{
				Config: providerConfig() + `
resource "xyops_webhook" "test" {
  id              = "tfaccwebhook"
  title           = "TF Acc Webhook Updated"
  url             = "https://httpbin.org/get"
  method          = "GET"
  headers         = jsonencode([{ name = "Accept", value = "application/json" }])
  body            = ""
  timeout         = 60
  retries         = 1
  follow          = true
  ssl_cert_bypass = true
  max_per_day     = 100
  notes           = "Managed by Terraform"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_webhook.test", "id", "tfaccwebhook"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "title", "TF Acc Webhook Updated"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "url", "https://httpbin.org/get"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "method", "GET"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "headers", `[{"name":"Accept","value":"application/json"}]`),
					resource.TestCheckResourceAttr("xyops_webhook.test", "timeout", "60"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "retries", "1"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "follow", "true"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "ssl_cert_bypass", "true"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "max_per_day", "100"),
				),
			},
			{
				ResourceName:            "xyops_webhook.test",
				ImportState:             true,
				ImportStateId:           "tfaccwebhook",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"body", "headers"},
			},
		},
	})
}

func TestAccWebhookResource_drift(t *testing.T) {
	config := providerConfig() + `
resource "xyops_webhook" "test" {
  id              = "tfaccwebhook"
  title           = "TF Acc Webhook Drift"
  url             = "https://httpbin.org/post"
  method          = "POST"
  headers         = jsonencode([])
  body            = "{}"
  timeout         = 30
  retries         = 0
  follow          = false
  ssl_cert_bypass = false
  max_per_day     = 0
  notes           = "Managed by Terraform"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("xyops_webhook.test", "id", "tfaccwebhook"),
				),
			},
			{
				PreConfig: func() {
					client := testAccClient(t)
					_ = client.Call("delete_web_hook", map[string]string{"id": "tfaccwebhook"}, nil)
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
					resource.TestCheckResourceAttr("xyops_webhook.test", "id", "tfaccwebhook"),
					resource.TestCheckResourceAttr("xyops_webhook.test", "title", "TF Acc Webhook Drift"),
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
