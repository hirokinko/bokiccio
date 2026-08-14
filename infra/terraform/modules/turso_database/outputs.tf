output "database_id" {
  description = "Turso database UUID."
  value       = turso_database.this.db_id
}

output "database_name" {
  description = "Stable Turso database name."
  value       = turso_database.this.name
}

output "hostname" {
  description = "Credential-free Turso database hostname."
  value       = turso_database.this.hostname
}

output "database_url" {
  description = "Credential-free libSQL URL for application configuration."
  value       = "libsql://${turso_database.this.hostname}"
}
