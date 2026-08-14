output "database_id" {
  description = "Turso demo database UUID."
  value       = module.database.database_id
}

output "database_name" {
  description = "Stable Turso demo database name."
  value       = module.database.database_name
}

output "hostname" {
  description = "Credential-free Turso demo database hostname."
  value       = module.database.hostname
}

output "database_url" {
  description = "Credential-free libSQL URL for demo application configuration."
  value       = module.database.database_url
}
