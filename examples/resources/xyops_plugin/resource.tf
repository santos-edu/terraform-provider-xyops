resource "xyops_plugin" "hello_shell" {
  title       = "Hello Shell"
  enabled     = true
  type        = "event"
  script_file = "#!/bin/sh\necho hello"
  command     = "sh"
  format      = "text"
  groups      = []
  params      = jsonencode([])
}
