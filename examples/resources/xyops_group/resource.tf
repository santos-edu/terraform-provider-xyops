resource "xyops_group" "all_servers" {
  title               = "All Servers"
  enabled             = true
  hostname_match      = ".*"
  sort_order          = 1
  max_jobs_per_server = 0
  notes               = "Managed by Terraform"
  alert_actions       = jsonencode([])
}
