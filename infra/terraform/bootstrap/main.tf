locals {
  deployment_service_account_id = "bokiccio-${var.environment_id}-deploy"
  turso_secret_id               = "bokiccio-${var.environment_id}-turso-token"
  secret_iam_manager_role_id    = "bokiccio_${replace(var.environment_id, "-", "_")}_secretIamManager"
  deployment_project_roles = toset([
    "roles/iam.serviceAccountAdmin",
    "roles/iap.admin",
    "roles/logging.logWriter",
    "roles/run.admin",
    "roles/serviceusage.serviceUsageConsumer",
  ])
  required_services = toset([
    "artifactregistry.googleapis.com",
    "cloudbuild.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "iap.googleapis.com",
    "run.googleapis.com",
    "secretmanager.googleapis.com",
    "serviceusage.googleapis.com",
    "storage.googleapis.com",
  ])
}

data "google_project" "current" {
  project_id = var.project_id
}

resource "google_project_service" "required" {
  for_each = local.required_services

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

resource "google_storage_bucket" "terraform_state" {
  name                        = var.state_bucket_name
  project                     = var.project_id
  location                    = var.state_bucket_location
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false
  labels                      = var.labels

  versioning {
    enabled = true
  }

  lifecycle {
    prevent_destroy = true
  }

  depends_on = [google_project_service.required["storage.googleapis.com"]]
}

resource "google_artifact_registry_repository" "containers" {
  project       = var.project_id
  location      = var.region
  repository_id = var.artifact_repository_id
  description   = "Bokiccio immutable container images"
  format        = "DOCKER"
  labels = merge(var.labels, {
    environment = "shared"
  })

  lifecycle {
    prevent_destroy = true
  }

  depends_on = [google_project_service.required["artifactregistry.googleapis.com"]]
}

resource "google_service_account" "deployment" {
  project      = var.project_id
  account_id   = local.deployment_service_account_id
  display_name = "Bokiccio ${var.environment_id} deployment"
  description  = "Runs explicit Bokiccio Cloud Build deployments and Terraform environment applies."

  depends_on = [google_project_service.required["iam.googleapis.com"]]
}

resource "google_service_account_iam_member" "cloud_build_impersonation" {
  service_account_id = google_service_account.deployment.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:service-${data.google_project.current.number}@gcp-sa-cloudbuild.iam.gserviceaccount.com"

  depends_on = [google_project_service.required["cloudbuild.googleapis.com"]]
}

resource "google_project_iam_member" "deployment" {
  for_each = local.deployment_project_roles

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.deployment.email}"

  depends_on = [google_project_service.required]
}

resource "google_storage_bucket_iam_member" "deployment_state" {
  bucket = google_storage_bucket.terraform_state.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.deployment.email}"
}

resource "google_artifact_registry_repository_iam_member" "deployment_writer" {
  project    = google_artifact_registry_repository.containers.project
  location   = google_artifact_registry_repository.containers.location
  repository = google_artifact_registry_repository.containers.repository_id
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.deployment.email}"
}

resource "google_secret_manager_secret" "turso" {
  project             = var.project_id
  secret_id           = local.turso_secret_id
  deletion_protection = true
  labels              = var.labels

  replication {
    auto {}
  }

  depends_on = [google_project_service.required["secretmanager.googleapis.com"]]
}

resource "google_project_iam_custom_role" "deployment_secret_iam_manager" {
  project     = var.project_id
  role_id     = local.secret_iam_manager_role_id
  title       = "Bokiccio ${var.environment_id} secret IAM manager"
  description = "Manages IAM policy only for the Bokiccio ${var.environment_id} Turso secret."
  permissions = [
    "secretmanager.secrets.getIamPolicy",
    "secretmanager.secrets.setIamPolicy",
  ]

  depends_on = [google_project_service.required["iam.googleapis.com"]]
}

resource "google_secret_manager_secret_iam_member" "deployment_secret_accessor" {
  project   = var.project_id
  secret_id = google_secret_manager_secret.turso.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.deployment.email}"
}

resource "google_secret_manager_secret_iam_member" "deployment_secret_iam_manager" {
  project   = var.project_id
  secret_id = google_secret_manager_secret.turso.secret_id
  role      = google_project_iam_custom_role.deployment_secret_iam_manager.name
  member    = "serviceAccount:${google_service_account.deployment.email}"
}
