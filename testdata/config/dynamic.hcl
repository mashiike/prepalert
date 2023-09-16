prepalert {
  required_version = ">=v0.12.0"
  sqs_queue_name   = "prepalert"
}

locals {
  default_message = <<EOF
This is a default message.
EOF
}
dynamic "rule" {
  for_each = ["ALB", "RDS", "Elasticache"]
  iterator = dynamic
  labels   = ["${lower(dynamic.value)}"]
  content {
    when = (webhook.org_name == "Macker...")
    update_alert {
      memo = local.default_message
    }
  }
}
