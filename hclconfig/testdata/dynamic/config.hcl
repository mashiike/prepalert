prepalert {
    required_version = ">=v0.11.0"
    sqs_queue_name   = "prepalert"
    service          = "prod"
}

dynamic "rule" {
    for_each = ["ALB", "RDS", "Elasticache"]
    iterator = dynamic
    labels  = ["${lower(dynamic.value)}"]
    content {
        alert {
            monitor_name = "${dynamic.value}"
        }

        information = <<EOF
How do you respond to alerts?
Describe information about your alert response here.
EOF
    }
}
