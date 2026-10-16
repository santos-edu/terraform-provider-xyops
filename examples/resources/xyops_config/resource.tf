resource "xyops_config" "global" {
  job_universal_actions = jsonencode({
    default  = []
    workflow = []
  })

  lifecycle {
    prevent_destroy = true
  }
}
