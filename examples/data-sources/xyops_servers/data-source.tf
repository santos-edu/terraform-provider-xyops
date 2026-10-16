data "xyops_servers" "all" {
  # Defaults to all records the instance kept (including offline servers).
  # Examples: groups:main, os_distro:ubuntu, os_arch:x86_64
  query = "*"
}
