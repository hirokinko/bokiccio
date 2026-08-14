mock_provider "turso" {
  mock_resource "turso_database" {
    defaults = {
      db_id    = "4a7599de-284d-4c26-a5d3-c7a21144c959"
      hostname = "bokiccio-demo.example.turso.io"
    }
  }
}

variables {
  organization_name = "example-organization"
  database_name     = "bokiccio-demo"
  group             = "default"
  size_limit        = "256mb"
}

run "database_contract" {
  command = apply

  assert {
    condition = (
      turso_database.this.organization_name == "example-organization" &&
      turso_database.this.name == "bokiccio-demo" &&
      turso_database.this.group == "default" &&
      turso_database.this.size_limit == "256mb"
    )
    error_message = "The database must retain its stable environment-derived identity and explicit capacity settings."
  }

  assert {
    condition = (
      output.database_id == "4a7599de-284d-4c26-a5d3-c7a21144c959" &&
      output.hostname == "bokiccio-demo.example.turso.io" &&
      output.database_url == "libsql://bokiccio-demo.example.turso.io"
    )
    error_message = "Outputs must contain only non-secret database identity and connection metadata."
  }
}

run "reject_invalid_database_name" {
  command = plan

  variables {
    database_name = "Bokiccio Demo"
  }

  expect_failures = [var.database_name]
}

run "reject_invalid_size_limit" {
  command = plan

  variables {
    size_limit = "unlimited"
  }

  expect_failures = [var.size_limit]
}
