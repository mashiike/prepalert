// Composition of the entire prepalert
prepalert {
  required_version = ">={{ .Version }}"
  sqs_queue_name   = "{{ .SQSQueueName }}" # Where to post the contents of the received webhook
  service          = "{{ .Service }}"      # The Mackerel service to which you want to post graph annotations
}

// Setup to post graph annotations describing fixed content no matter what alerts come in.
rule "simple" {
  alert {
    any = true
  }
  infomation = "How do you respond to alerts?"
}

// // Extensive configuration
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
//         BETWEEN '{{ "{{" }} .Alert.OpenedAt | to_time | strftime `%Y-%m-%d %H:%M:%S` {{ "}}" }}'::TIMESTAMP - interval '15 minutes'
//         AND '{{ "{{" }} .Alert.ClosedAt | to_time | strftime `%Y-%m-%d %H:%M:%S` {{ "}}" }}'
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
// {{ "{{" }}  index .QueryResults `alb_target_5xx_info` | to_table {{ "}}" }}
// EOT
// }