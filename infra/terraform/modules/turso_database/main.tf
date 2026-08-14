resource "turso_database" "this" {
  organization_name = var.organization_name
  name              = var.database_name
  group             = var.group
  size_limit        = var.size_limit

  lifecycle {
    prevent_destroy = true
  }
}
