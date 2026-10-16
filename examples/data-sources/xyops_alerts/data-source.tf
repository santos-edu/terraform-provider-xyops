# Example: alert invocations that are still active.
data "xyops_alerts" "active" {
  query = "active:true"
}
