resource "xyops_secret" "example" {
  title    = "Example Secret"
  enabled  = true
  notes    = "Managed by Terraform"
  value    = "change-me"
  category = []
  params   = jsonencode([])
  icon     = "key"
}
