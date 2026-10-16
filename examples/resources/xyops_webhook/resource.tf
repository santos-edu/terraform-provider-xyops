resource "xyops_webhook" "example" {
  title           = "Example Webhook"
  enabled         = true
  url             = "https://httpbin.org/post"
  method          = "POST"
  headers         = jsonencode([{ name = "Content-Type", value = "application/json" }])
  body            = jsonencode({ text = "alert from xyops" })
  timeout         = 30
  retries         = 0
  follow          = false
  ssl_cert_bypass = false
  max_per_day     = 0
  notes           = "Managed by Terraform"
}
