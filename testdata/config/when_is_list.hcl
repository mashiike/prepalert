prepalert {
    required_version = ">=v0.12.0"
    sqs_queue_name   = "prepalert"
}

locals {
    default_message =  <<EOF
How do you respond to alerts?
Describe information about your alert response here.
EOF
}

rule "simple" {
    when = [
        webhook.org_name == "Macker...",
        get_monitor(webhook.alert).id == "4gx...",
    ]
    update_alert {
        memo = local.default_message
    }
}
