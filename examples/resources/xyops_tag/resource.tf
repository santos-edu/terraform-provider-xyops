resource "xyops_tag" "production" {
  title   = "Production"
  enabled = true
  color   = "green"
  icon    = "tag"
  notes   = "Managed by Terraform"
}
