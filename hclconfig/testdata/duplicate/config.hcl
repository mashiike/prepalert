prepalert {
    required_version = ">=v0.2.0"
    sqs_queue_name   = "prepalert"
}

query_runner "redshift_data" "default" {
    database       = "dev"
    workgroup_name = "default"
}

query_runner "redshift_data" "default" {
    secrets_arn = "arn:aws:secretsmanager:<region>:<aws_account_id>:secret:<secret_name>"
}

query "alb_target_5xx_info" {
    runner = query_runner.redshift_data.default
    sql    = <<EOQ
SELECT *
FROM access_logs
LIMIT 1
EOQ
}

query "alb_target_5xx_info" {
    runner = query_runner.redshift_data.default
    sql    = <<EOQ
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

    infomation = <<EOF
How do you respond to alerts?
Describe information about your alert response here.
EOF
}
