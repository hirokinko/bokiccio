output "service_name" {
  description = "Configured demo Cloud Run service name."
  value       = module.environment.service_name
}

output "service_url" {
  description = "IAP-protected demo Cloud Run URL."
  value       = module.environment.service_url
}

output "iap_audience" {
  description = "Expected audience for demo IAP assertions."
  value       = module.environment.iap_audience
}

output "runtime_service_account_email" {
  description = "Stable demo runtime identity."
  value       = module.environment.runtime_service_account_email
}

output "container_image" {
  description = "Immutable image digest currently configured for the demo service."
  value       = module.environment.container_image
}
