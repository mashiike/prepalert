prepalert {
    required_version = ">=v0.2.0"
    sqs_queue_name   = "prepalert"
}

locals {
    default_message =  <<EOF
How do you respond to alerts?
Describe information about your alert response here.
EOF
}

rule "simple" {
    when = (runtime.event.org_name == "Macker...")
    update_alert {
        memo = local.default_message
    }
}
