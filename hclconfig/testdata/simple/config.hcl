prepalert {
    required_version = ">=v0.2.0"
    sqs_queue_name   = "prepalert"
    service          = "prod"
}

rule "simple" {
    alert {
        any = true
    }
    information = <<EOF
How do you respond to alerts?
Describe information about your alert response here.
EOF
}
