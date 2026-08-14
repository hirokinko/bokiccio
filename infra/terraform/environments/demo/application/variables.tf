variable "project_id" {
  description = "Existing Google Cloud project ID for the demo environment."
  type        = string
}

variable "region" {
  description = "Cloud Run region for the demo environment."
  type        = string
}

variable "environment_id" {
  description = "Stable identity for demo supporting resources."
  type        = string
  default     = "demo"
}

variable "service_name" {
  description = "Operator-selected Cloud Run service name; no private value is tracked in this root."
  type        = string
}

variable "container_image" {
  description = "Immutable Artifact Registry image reference including a sha256 digest."
  type        = string
}

variable "turso_database_url" {
  description = "Credential-free URL produced by the demo Turso root."
  type        = string
}

variable "turso_secret_id" {
  description = "Bootstrap-created Secret Manager secret ID containing the demo database token."
  type        = string
}

variable "turso_secret_version" {
  description = "Pinned numeric Secret Manager version created by the token bootstrap CLI."
  type        = string
}

variable "deployment_service_account_email" {
  description = "Bootstrap-created deployment service account email."
  type        = string
}

variable "iap_principals" {
  description = "Authoritative set of personal Google Accounts allowed through IAP."
  type        = set(string)
}

variable "max_instance_count" {
  description = "Maximum demo Cloud Run instance count."
  type        = number
  default     = 1
}

variable "deletion_protection" {
  description = "Protect the demo Cloud Run service from accidental deletion."
  type        = bool
  default     = true
}

variable "labels" {
  description = "Additional non-secret labels for demo resources."
  type        = map(string)
  default     = {}
}
