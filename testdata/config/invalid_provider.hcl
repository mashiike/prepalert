prepalert {
  required_version = ">=v0.12.0"
  sqs_queue_name   = "prepalert"

  auth {
    client_id     = "hoge"
    client_secret = "fuga"
  }
}

provider "invaliod" {}
