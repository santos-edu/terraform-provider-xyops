# Example: failed jobs in a date window.
data "xyops_jobs" "failures" {
  query = "tags:_error date:>=today"
}
