prepalert {
  required_version = ">=v0.12.0"
  sqs_queue_name   = "prepalert"

  s3_backend {
    bucket_name                 = must_env("PREPALERT_S3_BACKEND")
    object_key_prefix           = "prepalert/"
    viewer_base_url             = must_env("PREPALERT_VIEWER_BASE_URL")
    viewer_google_client_id     = must_env("GOOGLE_CLIENT_ID")
    viewer_google_client_secret = must_env("GOOGLE_CLIENT_SECRET")
    viewer_session_encrypt_key  = must_env("SESSION_ENCRYPT_KEY")
  }
}

locals {
    default_message =  <<EOF
How do you respond to alerts?
Describe information about your alert response here.
EOF
}

rule "simple" {
    // always triggerd
    when = true
    update_alert {
        memo = local.default_message
    }
}
