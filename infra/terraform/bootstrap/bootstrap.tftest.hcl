mock_provider "google" {
  mock_data "google_project" {
    defaults = {
      number = "123456789012"
    }
  }

}

run "bootstrap_security_contract" {
  command = plan

  variables {
    project_id             = "bokiccio-example"
    region                 = "asia-northeast1"
    environment_id         = "demo"
    state_bucket_name      = "bokiccio-example-terraform-state"
    state_bucket_location  = "ASIA"
    artifact_repository_id = "bokiccio"
    labels = {
      application = "bokiccio"
      environment = "demo"
    }
  }

  assert {
    condition = (
      google_storage_bucket.terraform_state.versioning[0].enabled &&
      google_storage_bucket.terraform_state.uniform_bucket_level_access &&
      google_storage_bucket.terraform_state.public_access_prevention == "enforced" &&
      !google_storage_bucket.terraform_state.force_destroy
    )
    error_message = "Terraform state must be versioned and protected from public or destructive access."
  }

  assert {
    condition = (
      google_artifact_registry_repository.containers.format == "DOCKER" &&
      google_artifact_registry_repository.containers.labels["environment"] == "shared" &&
      google_artifact_registry_repository_iam_member.deployment_writer.role == "roles/artifactregistry.writer"
    )
    error_message = "The shared repository must be labeled independently from one environment, and deployment access must be repository-scoped."
  }

  assert {
    condition = (
      google_service_account.deployment.account_id == "bokiccio-demo-deploy" &&
      google_storage_bucket_iam_member.deployment_state.role == "roles/storage.objectAdmin" &&
      google_secret_manager_secret.turso.secret_id == "bokiccio-demo-turso-token" &&
      google_secret_manager_secret.turso.deletion_protection &&
      output.turso_secret_id == "bokiccio-demo-turso-token"
    )
    error_message = "Deployment access must be tied to the stable environment identity and named resources."
  }

  assert {
    condition = (
      google_secret_manager_secret_iam_member.deployment_secret_accessor.role == "roles/secretmanager.secretAccessor" &&
      google_secret_manager_secret_iam_member.deployment_secret_accessor.secret_id == "bokiccio-demo-turso-token" &&
      google_secret_manager_secret_iam_member.deployment_secret_iam_manager.secret_id == "bokiccio-demo-turso-token" &&
      google_project_iam_custom_role.deployment_secret_iam_manager.role_id == "bokiccio_demo_secretIamManager" &&
      toset(google_project_iam_custom_role.deployment_secret_iam_manager.permissions) == toset([
        "secretmanager.secrets.getIamPolicy",
        "secretmanager.secrets.setIamPolicy",
      ])
    )
    error_message = "Deployment may access the selected secret and manage only that secret's IAM policy, without Secret Manager Admin."
  }

  assert {
    condition = (
      google_service_account_iam_member.cloud_build_impersonation.role == "roles/iam.serviceAccountTokenCreator" &&
      google_service_account_iam_member.cloud_build_impersonation.member == "serviceAccount:service-123456789012@gcp-sa-cloudbuild.iam.gserviceaccount.com"
    )
    error_message = "Only the Cloud Build service agent may impersonate the deployment service account."
  }

  assert {
    condition = (
      contains(local.required_services, "artifactregistry.googleapis.com") &&
      contains(local.required_services, "cloudbuild.googleapis.com") &&
      contains(local.required_services, "iap.googleapis.com") &&
      contains(local.required_services, "run.googleapis.com") &&
      contains(local.required_services, "secretmanager.googleapis.com")
    )
    error_message = "Bootstrap must retain every API required by image build and environment deployment."
  }
}
