variable "organization_name" {
  description = "Turso organization that owns the database."
  type        = string

  validation {
    condition     = length(trimspace(var.organization_name)) > 0
    error_message = "organization_name must not be empty."
  }
}

variable "database_name" {
  description = "Stable database name derived from the environment identity."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$", var.database_name))
    error_message = "database_name must be 1-64 lowercase letters, digits, or hyphens and must not end with a hyphen."
  }
}

variable "group" {
  description = "Existing Turso group for the database; null uses the organization default."
  type        = string
  default     = null

  validation {
    condition     = var.group == null || length(trimspace(var.group)) > 0
    error_message = "group must be null or a non-empty existing Turso group name."
  }
}

variable "size_limit" {
  description = "Optional Turso database size limit, for example 256mb or 1gb."
  type        = string
  default     = null

  validation {
    condition     = var.size_limit == null || can(regex("^[1-9][0-9]*(?:b|kb|mb|gb|tb)?$", lower(var.size_limit)))
    error_message = "size_limit must be null or a positive byte value such as 256mb or 1gb."
  }
}
