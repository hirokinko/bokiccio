module "database" {
  source = "../../../modules/turso_database"

  organization_name = var.organization_name
  database_name     = "bokiccio-${var.environment_id}"
  group             = var.group
  size_limit        = var.size_limit
}
