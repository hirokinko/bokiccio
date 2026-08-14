mock_provider "google" {
  mock_data "google_project" {
    defaults = {
      number = "123456789012"
    }
  }

  mock_resource "google_service_account" {
    defaults = {
      email = "bokiccio-demo-run@bokiccio-example.iam.gserviceaccount.com"
      name  = "projects/bokiccio-example/serviceAccounts/bokiccio-demo-run@bokiccio-example.iam.gserviceaccount.com"
    }
  }
}

mock_provider "time" {}

variables {
  project_id                       = "bokiccio-example"
  region                           = "asia-northeast1"
  service_name                     = "bokiccio-showcase"
  container_image                  = "asia-northeast1-docker.pkg.dev/bokiccio-example/bokiccio/application@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  turso_database_url               = "libsql://bokiccio-demo.example.turso.io"
  turso_secret_id                  = "bokiccio-demo-turso-token"
  turso_secret_version             = "1"
  deployment_service_account_email = "bokiccio-demo-deploy@bokiccio-example.iam.gserviceaccount.com"
  iap_principals                   = ["user:demo.operator@example.com"]
}

run "demo_root_contract" {
  command = apply

  assert {
    condition = (
      module.environment.service_name == "bokiccio-showcase" &&
      module.environment.runtime_service_account_email == "bokiccio-demo-run@bokiccio-example.iam.gserviceaccount.com"
    )
    error_message = "The demo root must keep the operator-selected service name separate from the stable demo runtime identity."
  }

  assert {
    condition = (
      module.environment.service_url == "https://bokiccio-showcase-123456789012.asia-northeast1.run.app" &&
      module.environment.iap_audience == "/projects/123456789012/locations/asia-northeast1/services/bokiccio-showcase" &&
      output.container_image == var.container_image
    )
    error_message = "The demo root must expose service-specific origin, IAP audience, and deployed image outputs."
  }

  assert {
    condition = (
      var.deletion_protection &&
      var.max_instance_count == 1 &&
      var.environment_id == "demo"
    )
    error_message = "The demo root must enable deletion protection and retain its low-cost identity defaults."
  }
}
