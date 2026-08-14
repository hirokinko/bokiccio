variable "project_id" {
  description = "Existing Google Cloud project ID."
  type        = string
}

variable "region" {
  description = "Cloud Run region."
  type        = string

  validation {
    condition     = can(regex("^[a-z]+-[a-z]+[0-9]$", var.region))
    error_message = "region must be a valid Google Cloud region such as asia-northeast1."
  }
}

variable "environment_id" {
  description = "Stable identifier for supporting resources; changing service_name does not change it."
  type        = string

  validation {
    condition     = can(regex("^[a-z]([a-z0-9-]{0,12}[a-z0-9])?$", var.environment_id))
    error_message = "environment_id must be 1-14 lowercase letters, digits, or hyphens, starting with a letter and ending with a letter or digit."
  }
}

variable "service_name" {
  description = "Cloud Run service name. It is intentionally independent from environment_id."
  type        = string

  validation {
    condition     = can(regex("^[a-z]([a-z0-9-]{0,47}[a-z0-9])?$", var.service_name))
    error_message = "service_name must be 1-49 lowercase letters, digits, or hyphens, starting with a letter and ending with a letter or digit."
  }
}

variable "container_image" {
  description = "Immutable Artifact Registry image reference including a sha256 digest."
  type        = string

  validation {
    condition     = can(regex("^[^[:space:]@]+@sha256:[0-9a-f]{64}$", var.container_image))
    error_message = "container_image must be an immutable image digest, not a tag."
  }
}

variable "turso_database_url" {
  description = "Credential-free libsql URL for this environment."
  type        = string

  validation {
    condition     = can(regex("^libsql://[A-Za-z0-9.-]+(?::[0-9]+)?$", var.turso_database_url))
    error_message = "turso_database_url must be a credential-free libsql URL without query or fragment data."
  }
}

variable "turso_secret_id" {
  description = "Existing Secret Manager secret ID containing the Turso token."
  type        = string

  validation {
    condition     = can(regex("^[A-Za-z0-9_-]{1,255}$", var.turso_secret_id))
    error_message = "turso_secret_id must be a Secret Manager secret ID."
  }
}

variable "turso_secret_version" {
  description = "Pinned numeric Secret Manager version containing the Turso token."
  type        = string

  validation {
    condition     = can(regex("^[1-9][0-9]*$", var.turso_secret_version))
    error_message = "turso_secret_version must be a fixed positive numeric version, not latest."
  }
}

variable "deployment_service_account_email" {
  description = "Bootstrap-created service account allowed to deploy this environment."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]@[a-z][a-z0-9-]{4,28}[a-z0-9]\\.iam\\.gserviceaccount\\.com$", var.deployment_service_account_email))
    error_message = "deployment_service_account_email must be a Google service account email."
  }
}

variable "iap_principals" {
  description = "Authoritative non-empty set of personal Google Account principals."
  type        = set(string)

  validation {
    condition = length(var.iap_principals) > 0 && alltrue([
      for principal in var.iap_principals : can(regex("^user:[^[:space:]@]+@[^[:space:]@]+$", principal))
    ])
    error_message = "iap_principals must contain at least one user:<email> principal and no public or group members."
  }
}

variable "max_instance_count" {
  description = "Maximum Cloud Run instance count; zero minimum preserves scale-to-zero."
  type        = number
  default     = 1

  validation {
    condition     = var.max_instance_count >= 1 && var.max_instance_count <= 10 && floor(var.max_instance_count) == var.max_instance_count
    error_message = "max_instance_count must be an integer from 1 through 10."
  }
}

variable "deletion_protection" {
  description = "Protect the Cloud Run service from accidental Terraform deletion."
  type        = bool
}

variable "labels" {
  description = "Non-secret labels applied to environment resources."
  type        = map(string)
  default     = {}
}
