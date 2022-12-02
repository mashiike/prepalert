prepalert {
  required_version = ">=v0.3.1"
  sqs_queue_name   = "prepalert"
  service          = "prepalert-dev"

  s3_backend {
    bucket_name                 = must_env("PREPALERT_S3_BACKEND")
    object_key_prefix           = "prepalert/"
    viewer_base_url             = must_env("PREPALERT_VIEWER_BASE_URL")
    viewer_google_client_id     = must_env("GOOGLE_CLIENT_ID")
    viewer_google_client_secret = must_env("GOOGLE_CLIENT_SECRET")
    viewer_session_encrypt_key  = must_env("SESSION_ENCRYPT_KEY")
  }
}

rule "simple" {
  alert {
    any = true
  }
  information = "How do you respond to alerts?"
}
