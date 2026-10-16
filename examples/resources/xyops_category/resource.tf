resource "xyops_category" "general" {
  title   = "General"
  enabled = true
  color   = "blue"
  icon    = "folder"
  notes   = "Managed by Terraform"
  actions = jsonencode([])
  limits  = jsonencode([])
}
