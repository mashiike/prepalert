prepalert {
  required_version = ">=v0.2.0"
  sqs_queue_name   = "prepalert"
  service          = "prod"

  s3_backend {
    # Upload information about alerts to S3 and set up a simplified view of the uploaded information.
    bucket_name                 = "prepalert-infomation"
    object_key_prefix           = "alerts/"
    viewer_base_url             = "http://localhost:8080"
    viewer_google_client_id     = env("GOOGLE_CLIENT_ID", "")
    viewer_google_client_secret = env("GOOGLE_CLIENT_SECRET", "")
    viewer_session_encrypt_key  = env("SESSION_ENCRYPT_KEY", "")
  }
}

rule "simple" {
  alert {
    any = true
  }
  infomation = <<EOF
How do you respond to alerts?
Describe information about your alert response here.
EOF
}
