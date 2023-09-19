prepalert {
  required_version = ">=v0.12.0"
  sqs_queue_name   = "prepalert"

  backend "s3" {
    bucket_name                 = must_env("PREPALERT_S3_BUCKET")
    object_key_prefix           = "prepalert/alerts/"
    viewer_base_url             = must_env("PREPALERT_VIEWER_BASE_URL")
    viewer_google_client_id     = must_env("GOOGLE_CLIENT_ID")
    viewer_google_client_secret = must_env("GOOGLE_CLIENT_SECRET")
    viewer_session_encrypt_key  = must_env("SESSION_ENCRYPT_KEY")
  }
}

provider "cloudwatch_logs_insights" {
  region = "ap-northeast-1"
}

query "cloudwatch_logs_insights" "lambda" {
  log_group_names = [
    "/aws/lambda/prepalert",
  ]
  query      = "fields @timestamp, @message | limit 10"
  start_time = webhook.alert.opened_at - duration("15m")
  end_time   = coalesce(webhook.alert.closed_at, now())
}

locals {
  default_message = <<EOF
How do you respond to alerts?
Describe information about your alert response here.
EOF
}

rule "simple" {
  // always triggerd
  when = true
  update_alert {
    memo = "${local.default_message}\n${result_to_jsonlines(query.cloudwatch_logs_insights.lambda)}"
  }
  post_graph_annotation {
    service = "prepalert"
  }
}
