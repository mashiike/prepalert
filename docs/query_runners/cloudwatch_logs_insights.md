## Feature: Cloudwatch logs insights Query runner

sample configuration

```
prepalert {
  required_version = ">=v0.6.0"
  service          = "prod"
  sqs_queue_name   = "prepalert"
}

query_runner "cloudwatch_logs_insights" "default" {
  region = "ap-northeast-1"
}

query "cw_logs" {
  runner = query_runner.cloudwatch_logs_insights.default
  start_time = strftime_in_zone("%Y-%m-%dT%H:%M:%S%z", "UTC", runtime.event.alert.opened_at - duration("15m"))
  query  = <<EOT
fields @timestamp, @message
| sort @timestamp desc
| limit 20
EOT
  log_group_names = [
    "<your log group name>"
  ]
}

rule "cloudwatch_test" {
  alert {
    any = true
  }
  queries = [
    query.cw_logs,
  ]
  information = runtime.query_result.cw_logs.vertical_table
}
```

