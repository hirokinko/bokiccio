module "environment" {
  source = "../../../modules/bokiccio_environment"

  project_id                       = var.project_id
  region                           = var.region
  environment_id                   = var.environment_id
  service_name                     = var.service_name
  container_image                  = var.container_image
  turso_database_url               = var.turso_database_url
  turso_secret_id                  = var.turso_secret_id
  turso_secret_version             = var.turso_secret_version
  deployment_service_account_email = var.deployment_service_account_email
  iap_principals                   = var.iap_principals
  max_instance_count               = var.max_instance_count
  deletion_protection              = var.deletion_protection
  labels = merge(
    {
      application = "bokiccio"
      environment = var.environment_id
    },
    var.labels,
  )
}
