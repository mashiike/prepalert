prepalert {
    required_version = ">=v0.2.0"
    sqs_queue_name   = "prepalert"
    service          = "prod"
}

query_runner "redshift_data" "default" {
    cluster_identifier = "warehouse"
    database           = "dev"
    db_user            = "admin"
}

query "alb_target_5xx_info" {
    runner = query_runner.redshift_data.default
    sql = <<EOQ
SELECT *
FROM access_logs
LIMIT 1
EOQ
}

rule "alb_target_5xx" {
    alert {
        monitor_name = "ALB Target 5xx"
    }

    queries = [
        query.alb_target_5xx_info,
    ]
    params = {
        version = var.version,
        hoge    = "hoge",
    }

    information = <<EOT
5xx info:
${runtime.query_result.alb_target_5xx_info.table}
EOT
    update_alert_memo     = false
    post_graph_annotation = false
}
