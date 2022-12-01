// Composition of the entire prepalert
prepalert {
  required_version = ">={{ .Version }}"
  sqs_queue_name   = "{{ .SQSQueueName }}" # Where to post the contents of the received webhook
  service          = "{{ .Service }}"      # The Mackerel service to which you want to post graph annotations

  //   //Comment out the following if you want to set up Basic Authentication for webhooks
  //   auth {
  //     // The actual setting values are read from environment variables
  //     client_id     = must_env("PREPALERT_BASIC_USER")
  //     client_secret = env("PREPALERT_BASIC_PASS")
  //   }
}

// Setup to post graph annotations describing fixed content no matter what alerts come in.
rule "simple" {
  alert {
    any = true
  }
  infomation            = "How do you respond to alerts?"
  update_alert_memo     = true
  post_graph_annotation = true
}

// // Advanced configuration
// // Query Redshift and embed the results in graph annotations.
//
// query_runner "redshift_data" "default" {
//   cluster_identifier = "warehouse"
//   database           = "dev"
//   db_user            = "admin"
// }
//
// query "alb_target_5xx_info" {
//   runner = query_runner.redshift_data.default
//   sql    = {{ "<<" -}}EOQ
// SELECT
//     mthod, path, count(*) as cnt
// FROM access_logs
// WHERE
//     access_at
//         BETWEEN '${strftime("%Y-%m-%d %H:%M:%S",runtime.event.alert.opened_at)}'::TIMESTAMP - interval '15 minutes'
//         AND '${strftime("%Y-%m-%d %H:%M:%S",runtime.event.alert.closed_at)}'
//     status BETWEEN 500 AND 599
// GROUP BY 1,2
// ORDER BY 3 desc LIMIT 5
// EOQ
// }
//
// rule "alb_target_5xx" {
//   alert {
//     monitor_name = "ALB Target 5xx"
//   }
//
//   queries = [
//     query.alb_target_5xx_info,
//   ]
//
//   infomation = {{ "<<" }}EOT
// 5xx info:
// ${runtime.query_result.alb_target_5xx_info.table}
// EOT
// }
