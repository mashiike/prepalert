prepalert {
  required_version = ">=v0.12.0"
  sqs_queue_name   = "prepalert"

  plugins {
    http = {
      cmd         = "go run cmd/example-http-csv-plugin/main.go"
      sync_output = true
    }
  }
}

provider "http" {
  endpoint = must_env("TEST_SERVER_ENDPOINT")
}

query "http" "test_server" {
  fields = ["id", "name"]
  limit  = 5
}

rule "test_application_error" {
  when = true
  update_alert {
    memo = "${query.http.test_server.result.query}\n${result_to_table(query.http.test_server)}"
  }
}
