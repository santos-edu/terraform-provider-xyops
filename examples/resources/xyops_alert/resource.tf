resource "xyops_alert" "high_cpu" {
  title      = "High CPU load"
  expression = "cpu.currentLoad > 80"
  message    = "CPU load is {{cpu.currentLoad}}%"
  enabled    = true
  max_count  = 1
  notes      = "Managed by Terraform"
  groups     = []
  actions    = jsonencode([])
}
