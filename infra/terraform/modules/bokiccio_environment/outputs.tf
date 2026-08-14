output "service_name" {
  description = "Configured Cloud Run service name."
  value       = google_cloud_run_v2_service.application.name
}

output "service_url" {
  description = "Deterministic IAP-protected Cloud Run origin."
  value       = local.service_origin
}

output "iap_audience" {
  description = "Expected audience for signed Cloud Run IAP assertions."
  value       = local.iap_audience
}

output "runtime_service_account_email" {
  description = "Stable runtime identity, independent of service_name."
  value       = google_service_account.runtime.email
}

output "container_image" {
  description = "Immutable image digest currently configured for the Cloud Run service."
  value       = google_cloud_run_v2_service.application.template[0].containers[0].image
}
