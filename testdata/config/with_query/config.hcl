prepalert {
  required_version = ">=v0.12.0"
  sqs_queue_name   = "prepalert"

  auth {
    client_id     = "hoge"
    client_secret = "fuga"
  }

  retry {
    interval       = duration("10s")
    backoff_factor = 4
    max_interval   = duration("5m")
    jitter         = duration("30s")
  }

}

provider "redshift_data" {
  cluster_identifier = "warehouse"
  database           = "dev"
  db_user            = "admin"
}

provider "redshift_data" {
  ailias         = "serverless"
  workgroup_name = "default"
  database       = "dev"
}
