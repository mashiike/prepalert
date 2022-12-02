
prepalert {
  required_version = ">=v0.2.0"
  service          = "prod"
  sqs_queue_name   = "prepalert"

  auth {
    client_id     = "hoge"
    client_secret = "hoge"
  }
}

query_runner "redshift_data" "default" {
  cluster_identifier = "warehouse"
  database           = "dev"
  db_user            = "warehouse"
}

query "access_data" {
  runner = query_runner.redshift_data.default
  sql    = ""
}

rule "any" {
  alert {
    any = true
  }
  information = "hoge"
}
