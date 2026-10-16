resource "xyops_channel" "ops" {
  title     = "Ops"
  enabled   = true
  notes     = "Managed by Terraform"
  users     = []
  email     = "ops@example.com"
  web_hook  = ""
  run_event = ""
  sound     = "attention-3.mp3"
  icon      = "bell"
}
