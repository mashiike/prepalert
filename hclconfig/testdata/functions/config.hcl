prepalert {
    required_version = ">=v0.2.0"
    sqs_queue_name   = "prepalert-${var.version}"
    service          = must_env("TEST_ENV")

    auth {
        client_id     = env("BASIC_CLIENT_ID")
        client_secret = env("BASIC_CLIENT_SECRETS")
    }
}

query_runner "redshift_data" "default" {
    cluster_identifier = env("TEST_CLUSTER")
    database           = must_env("TEST_ENV")
    db_user            = "admin"
}

query "alb_target_5xx_info" {
    runner = query_runner.redshift_data.default
    sql = file("./query.sql")
}

rule "alb_target_5xx" {
    alert {
        monitor_name = "ALB Target 5xx"
    }

    queries = [
        query.alb_target_5xx_info,
    ]

    infomation = file("./infomation_template.txt")
}

rule "constant" {
    alert {
        monitor_id = "xxxxxxxxxxxx"
    }
    infomation = "prepalert: ${var.version}"
}
