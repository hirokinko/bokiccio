data "google_project" "current" {
  project_id = var.project_id
}

locals {
  runtime_service_account_id = "bokiccio-${var.environment_id}-run"
  iap_audience               = "/projects/${data.google_project.current.number}/locations/${var.region}/services/${var.service_name}"
  service_origin             = "https://${var.service_name}-${data.google_project.current.number}.${var.region}.run.app"
  iap_service_agent          = "serviceAccount:service-${data.google_project.current.number}@gcp-sa-iap.iam.gserviceaccount.com"
}

resource "google_service_account" "runtime" {
  project      = var.project_id
  account_id   = local.runtime_service_account_id
  display_name = "Bokiccio ${var.environment_id} runtime"
  description  = "Runtime identity for the Bokiccio ${var.environment_id} environment."
}

resource "google_service_account_iam_member" "deployment_act_as_runtime" {
  service_account_id = google_service_account.runtime.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${var.deployment_service_account_email}"
}

resource "google_secret_manager_secret_iam_member" "runtime_accessor" {
  project   = var.project_id
  secret_id = var.turso_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_cloud_run_v2_service" "application" {
  project             = var.project_id
  name                = var.service_name
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  iap_enabled         = true
  deletion_protection = var.deletion_protection
  labels              = var.labels

  template {
    service_account = google_service_account.runtime.email

    scaling {
      min_instance_count = 0
      max_instance_count = var.max_instance_count
    }

    containers {
      image = var.container_image

      ports {
        container_port = 8080
      }

      env {
        name  = "TURSO_DATABASE_URL"
        value = var.turso_database_url
      }
      env {
        name = "TURSO_AUTH_TOKEN"
        value_source {
          secret_key_ref {
            secret  = var.turso_secret_id
            version = var.turso_secret_version
          }
        }
      }
      env {
        name  = "BOKICCIO_IAP_AUDIENCE"
        value = local.iap_audience
      }
      env {
        name  = "BOKICCIO_EXTERNAL_ORIGIN"
        value = local.service_origin
      }
      env {
        name  = "BOKICCIO_ENVIRONMENT"
        value = "production"
      }

      startup_probe {
        initial_delay_seconds = 0
        timeout_seconds       = 3
        period_seconds        = 5
        failure_threshold     = 12

        http_get {
          path = "/livez"
          port = 8080
        }
      }

      liveness_probe {
        initial_delay_seconds = 0
        timeout_seconds       = 3
        period_seconds        = 10
        failure_threshold     = 3

        http_get {
          path = "/livez"
          port = 8080
        }
      }
    }
  }

  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    google_service_account_iam_member.deployment_act_as_runtime,
    google_secret_manager_secret_iam_member.runtime_accessor,
  ]
}

resource "google_cloud_run_v2_service_iam_binding" "iap_invoker" {
  project  = google_cloud_run_v2_service.application.project
  location = google_cloud_run_v2_service.application.location
  name     = google_cloud_run_v2_service.application.name
  role     = "roles/run.invoker"
  members  = [local.iap_service_agent]
}

resource "google_iap_web_cloud_run_service_iam_binding" "users" {
  project                = google_cloud_run_v2_service.application.project
  location               = google_cloud_run_v2_service.application.location
  cloud_run_service_name = google_cloud_run_v2_service.application.name
  role                   = "roles/iap.httpsResourceAccessor"
  members                = var.iap_principals

  depends_on = [google_cloud_run_v2_service_iam_binding.iap_invoker]
}
