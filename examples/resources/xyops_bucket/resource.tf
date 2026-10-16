resource "xyops_bucket" "config" {
  title   = "Terraform Config"
  enabled = true
  notes   = "Managed by Terraform"
  data    = jsonencode({ environment = "prod" })
}
