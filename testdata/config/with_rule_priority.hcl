prepalert {
    required_version = ">=v0.12.0"
    sqs_queue_name   = "prepalert"
}

rule "second" {
    when = true
    priority = 50
    update_alert {
        memo = "hoge"
    }
}

rule "first" {
    when = true
    priority = 100
    update_alert {
        memo = "fuga"
    }
}
