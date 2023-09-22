query "redshift_data" "access_logs" {
  sql = "SELECT * FROM access_logs LIMIT 1"
}

query "redshift_data" "serverless_access_logs" {
  provider = redshift_data.serverless
  sql      = "SELECT * FROM access_logs LIMIT 1"
}

rule "alb_target_5xx" {
  when = has_prefix(webhook.alert.monitor_name, "Monitor")
  update_alert {
    memo = <<EOF
this is access_logs:
${result_to_table(query.redshift_data.access_logs)}
EOF
  }
  post_graph_annotation {
    service = "prod"
    additional_description = <<EOF
this is access_logs:
${result_to_table(query.redshift_data.access_logs)}
EOF
  }
}

rule "alb_target_5xx_for_serverless" {
  when = has_prefix(webhook.alert.monitor_name, "Serverless Data")
  update_alert {
    memo = <<EOF
this is serverless_access_logs:
${result_to_jsonlines(query.redshift_data.serverless_access_logs)}
EOF
  }
}
