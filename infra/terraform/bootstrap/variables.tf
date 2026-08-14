variable "project_id" {
  description = "Existing Google Cloud project ID."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.project_id))
    error_message = "project_id must be a valid Google Cloud project ID."
  }
}

variable "region" {
  description = "Google Cloud region used by Artifact Registry and the environment."
  type        = string

  validation {
    condition     = can(regex("^[a-z]+-[a-z]+[0-9]$", var.region))
    error_message = "region must be a valid Google Cloud region such as asia-northeast1."
  }
}

variable "environment_id" {
  description = "Stable, non-secret identifier used for supporting resource names."
  type        = string

  validation {
    condition     = can(regex("^[a-z]([a-z0-9-]{0,12}[a-z0-9])?$", var.environment_id))
    error_message = "environment_id must be 1-14 lowercase letters, digits, or hyphens, starting with a letter and ending with a letter or digit."
  }
}

variable "state_bucket_name" {
  description = "Globally unique GCS bucket name for environment Terraform state."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9._-]{1,61}[a-z0-9]$", var.state_bucket_name))
    error_message = "state_bucket_name must be a valid 3-63 character GCS bucket name."
  }
}

variable "state_bucket_location" {
  description = "GCS location for Terraform state, for example ASIA or asia-northeast1."
  type        = string

  validation {
    condition     = length(trimspace(var.state_bucket_location)) > 0
    error_message = "state_bucket_location must not be empty."
  }
}

variable "artifact_repository_id" {
  description = "Artifact Registry repository ID for Bokiccio container images."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{0,61}[a-z0-9]$", var.artifact_repository_id))
    error_message = "artifact_repository_id must be 2-63 lowercase letters, digits, or hyphens."
  }
}

variable "turso_secret_id" {
  description = "Existing Secret Manager secret ID containing the environment-specific Turso token."
  type        = string

  validation {
    condition     = can(regex("^[A-Za-z0-9_-]{1,255}$", var.turso_secret_id))
    error_message = "turso_secret_id must be a Secret Manager secret ID, not a secret payload or resource URL."
  }
}

variable "labels" {
  description = "Non-secret labels applied to bootstrap resources."
  type        = map(string)
  default     = {}
}
