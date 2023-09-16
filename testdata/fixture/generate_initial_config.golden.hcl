// Composition of the entire prepalert
prepalert {
  required_version = ">=v0.12.0"
  sqs_queue_name   = "test-sqs" # Where to post the contents of the received webhook

  // if you want to Basic Authentication to the webhook endpoint, uncomment the following
  //   auth {
  //     // The actual setting values are read from environment variables
  //     client_id     = must_env("PREPALERT_BASIC_USER")
  //     client_secret = env("PREPALERT_BASIC_PASS")
  //   }
}

locals {
  default_message = "How do you respond to alerts?"
}

// Setup to action `update_alert` fixed memo no matter what alerts come in.

rule "simple" {
  when = (webhook.org_name == "test-org")
  update_alert {
    memo = local.default_message
  }
}

// // Advanced configuration
// // Query Redshift and embed the results in graph annotations.
//
// provider "redshift_data" {
//   cluster_identifier = "warehouse"
//   database           = "dev"
//   db_user            = "admin"
// }
//
// query "redshift_data" "access_count" {
//   sql    = <<EOQ
// SELECT
//     mthod, path, count(*) as cnt
// FROM access_logs
// WHERE
//     access_at
//         BETWEEN '${strftime("%Y-%m-%d %H:%M:%S",webhook.alert.opened_at)}'::TIMESTAMP - interval '15 minutes'
//         AND '${strftime("%Y-%m-%d %H:%M:%S",webhook.alert.closed_at)}'
//     status BETWEEN 500 AND 599
// GROUP BY 1,2
// ORDER BY 3 desc LIMIT 5
// EOQ
// }
//
// rule "with_query" {
//   when = (get_monitor(webhook.alert).id == "48xe....")
//   upldate_alert {
//     memo = <<EOT
// 5xx info:
// ${result_to_table(query.redshift_data.access_count)}
// EOT
// }
