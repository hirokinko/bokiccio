terraform {
  required_version = "~> 1.15.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 7.40.0"
    }
    time = {
      source  = "hashicorp/time"
      version = "0.14.0"
    }
  }

  backend "gcs" {}
}
