prepalert {
    required_version = ">=v0.2.0"
    sqs_queue_name   = "prepalert"
    service          = "prod"

    s3_backend {
        bucket_name = "prepalert-infomation"
        object_key_prefix = "alerts/"
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
