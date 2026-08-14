mock_provider "google" {
  mock_data "google_project" {
    defaults = {
      number = "123456789012"
    }
  }
}

variables {
  project_id                       = "bokiccio-example"
  region                           = "asia-northeast1"
  environment_id                   = "demo"
  service_name                     = "bokiccio-demo"
  container_image                  = "asia-northeast1-docker.pkg.dev/bokiccio-example/bokiccio/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  turso_database_url               = "libsql://bokiccio-demo.example.turso.io"
  turso_secret_id                  = "bokiccio-demo-turso-token"
  turso_secret_version             = "3"
  deployment_service_account_email = "bokiccio-demo-deploy@bokiccio-example.iam.gserviceaccount.com"
  iap_principals                   = ["user:demo.operator@example.com"]
  deletion_protection              = false
  labels = {
    application = "bokiccio"
    environment = "demo"
  }
}

run "environment_security_contract" {
  command = plan

  assert {
    condition = (
      google_cloud_run_v2_service.application.iap_enabled &&
      google_cloud_run_v2_service.application.ingress == "INGRESS_TRAFFIC_ALL" &&
      google_cloud_run_v2_service.application.template[0].scaling[0].min_instance_count == 0 &&
      google_cloud_run_v2_service.application.template[0].scaling[0].max_instance_count == 1
    )
    error_message = "Cloud Run must use direct IAP and preserve scale-to-zero with an explicit maximum."
  }

  assert {
    condition = (
      google_cloud_run_v2_service_iam_binding.iap_invoker.members == toset(["serviceAccount:service-123456789012@gcp-sa-iap.iam.gserviceaccount.com"]) &&
      !contains(google_cloud_run_v2_service_iam_binding.iap_invoker.members, "allUsers") &&
      !contains(google_cloud_run_v2_service_iam_binding.iap_invoker.members, "allAuthenticatedUsers")
    )
    error_message = "The authoritative Cloud Run invoker binding must contain only the IAP service agent."
  }

  assert {
    condition = (
      google_iap_web_cloud_run_service_iam_binding.users.role == "roles/iap.httpsResourceAccessor" &&
      google_iap_web_cloud_run_service_iam_binding.users.members == toset(["user:demo.operator@example.com"])
    )
    error_message = "IAP access must be authoritative and limited to the configured personal accounts."
  }

  assert {
    condition = (
      google_cloud_run_v2_service.application.template[0].containers[0].image == var.container_image &&
      one([for environment in google_cloud_run_v2_service.application.template[0].containers[0].env : environment if environment.name == "TURSO_AUTH_TOKEN"]).value_source[0].secret_key_ref[0].version == "3" &&
      google_secret_manager_secret_iam_member.runtime_accessor.role == "roles/secretmanager.secretAccessor"
    )
    error_message = "Runtime configuration must use an immutable image and pinned resource-scoped secret access."
  }

  assert {
    condition = (
      output.service_url == "https://bokiccio-demo-123456789012.asia-northeast1.run.app" &&
      output.iap_audience == "/projects/123456789012/locations/asia-northeast1/services/bokiccio-demo" &&
      google_service_account.runtime.account_id == "bokiccio-demo-run"
    )
    error_message = "Audience and origin follow service_name while the runtime identity follows environment_id."
  }
}

run "service_rename_preserves_supporting_identity" {
  command = plan

  variables {
    service_name = "bokiccio-showcase"
  }

  assert {
    condition = (
      google_cloud_run_v2_service.application.name == "bokiccio-showcase" &&
      output.service_url == "https://bokiccio-showcase-123456789012.asia-northeast1.run.app" &&
      google_service_account.runtime.account_id == "bokiccio-demo-run" &&
      google_secret_manager_secret_iam_member.runtime_accessor.secret_id == "bokiccio-demo-turso-token"
    )
    error_message = "A service rename must not rename the runtime identity or secret reference."
  }
}

run "reject_tagged_image" {
  command = plan

  variables {
    container_image = "asia-northeast1-docker.pkg.dev/bokiccio-example/bokiccio/app:latest"
  }

  expect_failures = [var.container_image]
}

run "reject_latest_secret" {
  command = plan

  variables {
    turso_secret_version = "latest"
  }

  expect_failures = [var.turso_secret_version]
}

run "reject_empty_iap_principals" {
  command = plan

  variables {
    iap_principals = []
  }

  expect_failures = [var.iap_principals]
}
