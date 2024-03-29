prepalert {
  required_version = ">=v0.12.0"
  sqs_queue_name   = "prepalert"

  plugins {
    test = {
      cmd         = "go run testdata/plugin/testplugin/main.go"
      sync_output = true
    }
  }
}

provider "test" {
  magic = "this is test"
}

query "test" "hoge" {
  code = "hoge"
  details {
    description = "test hoge query"
  }
}

rule "test_application_error" {
  when = true
  update_alert {
    memo = result_to_table(query.test.hoge)
  }
}
