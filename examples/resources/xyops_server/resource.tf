# xyops_server adopts a server xyOps already knows about; it never creates one.
# The agent has to register the server before this resource can adopt it.
resource "xyops_server" "local" {
  id      = "local"
  enabled = true
  notes   = "Managed by Terraform (adopted)"
}
