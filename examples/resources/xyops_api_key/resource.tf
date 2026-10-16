resource "xyops_api_key" "automation" {
  title       = "Automation"
  description = "Managed by Terraform"
  active      = true
  privileges = {
    "admin" = 0
  }
  roles = []
}
