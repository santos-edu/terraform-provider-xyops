resource "xyops_event" "hello_event" {
  title    = "Hello Event"
  enabled  = true
  category = "general"
  plugin   = "shellplug"
  targets  = ["local"]
  algo     = "random"
  notes    = "Managed by Terraform"
  params   = jsonencode({ script = "#!/bin/sh\necho hello" })
  fields   = jsonencode([])
  triggers = jsonencode([])
  limits   = jsonencode([])
  actions  = jsonencode([])
}
