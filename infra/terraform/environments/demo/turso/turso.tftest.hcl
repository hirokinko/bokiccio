mock_provider "turso" {
  mock_resource "turso_database" {
    defaults = {
      db_id    = "4a7599de-284d-4c26-a5d3-c7a21144c959"
      hostname = "bokiccio-demo.example.turso.io"
    }
  }
}

variables {
  turso_api_token   = "test-token-not-used-by-mock-provider"
  organization_name = "example-organization"
  size_limit        = "256mb"
}

run "demo_database_identity" {
  command = apply

  assert {
    condition = (
      module.database.database_name == "bokiccio-demo" &&
      output.database_url == "libsql://bokiccio-demo.example.turso.io"
    )
    error_message = "The demo root must derive database identity from environment_id and expose only a credential-free URL."
  }
}
