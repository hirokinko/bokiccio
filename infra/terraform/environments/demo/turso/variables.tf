variable "turso_api_token" {
  description = "Short-lived Turso organization API token used only to configure the provider."
  type        = string
  sensitive   = true
  ephemeral   = true
}

variable "organization_name" {
  description = "Turso organization that owns the demo database."
  type        = string

  validation {
    condition     = length(trimspace(var.organization_name)) > 0
    error_message = "organization_name must not be empty."
  }
}

variable "environment_id" {
  description = "Stable environment identity used to derive the Turso database name."
  type        = string
  default     = "demo"

  validation {
    condition     = can(regex("^[a-z]([a-z0-9-]{0,12}[a-z0-9])?$", var.environment_id))
    error_message = "environment_id must be 1-14 lowercase letters, digits, or hyphens, starting with a letter and ending with a letter or digit."
  }
}

variable "group" {
  description = "Existing Turso group for the demo database; null uses the organization default."
  type        = string
  default     = null
}

variable "size_limit" {
  description = "Optional Turso database size limit, for example 256mb."
  type        = string
  default     = null
}
