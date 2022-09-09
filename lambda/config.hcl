prepalert {
  required_version = ">=v0.3.1"
  sqs_queue_name   = "prepalert"
  service          = "prepalert-dev"

  s3_backend {
    bucket_name       = must_env("PREPALERT_S3_BACKEND")
    object_key_prefix = "prepalert/"
    viewer_base_url   = must_env("PREPALERT_VIEWER_BASE_URL")
  }
}

rule "simple" {
  alert {
    any = true
  }
  infomation = "How do you respond to alerts?"
}
