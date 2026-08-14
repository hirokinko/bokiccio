output "state_bucket_name" {
  description = "GCS bucket to configure as the environment backend."
  value       = google_storage_bucket.terraform_state.name
}

output "artifact_repository" {
  description = "Artifact Registry Docker repository path without an image name."
  value       = "${google_artifact_registry_repository.containers.location}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.containers.repository_id}"
}

output "deployment_service_account_email" {
  description = "Service account used by explicit Cloud Build deployments."
  value       = google_service_account.deployment.email
}

output "turso_secret_id" {
  description = "Secret Manager secret ID whose payload is populated outside Terraform."
  value       = google_secret_manager_secret.turso.secret_id
}

output "enabled_services" {
  description = "Google Cloud APIs retained when the bootstrap stack is destroyed."
  value       = sort(tolist(local.required_services))
}
