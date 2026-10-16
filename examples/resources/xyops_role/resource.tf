resource "xyops_role" "operator" {
  title   = "Operator"
  enabled = true
  notes   = "Managed by Terraform"
  privileges = {
    "view"   = 1
    "create" = 0
  }
  categories = []
}
