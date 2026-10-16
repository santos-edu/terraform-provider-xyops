resource "xyops_workflow" "example" {
  title    = "Example Workflow"
  enabled  = true
  category = "general"
  tags     = []
  triggers = jsonencode([])
  actions  = jsonencode([])
  limits   = jsonencode([])
  workflow = jsonencode({
    nodes = [
      { id = "start", type = "event", event = "shellplug", x = 10, y = 10 }
    ]
    connections = []
  })
}
