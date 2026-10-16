resource "xyops_monitor" "cpu_load" {
  title          = "CPU Load"
  source         = "cpu.currentLoad"
  data_type      = "float"
  display        = true
  suffix         = "%"
  min_vert_scale = 100
  max_vert_scale = 500
  delta          = false
  divide_time    = false
  zero_min       = 0
  groups         = []
  notes          = "Managed by Terraform"
}
