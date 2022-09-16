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
  start_time = "{{ .Alert.OpenedAt | to_time | add_time `-15m` | strftime_in_zone `%Y-%m-%dT%H:%M:%S%z` `UTC`  }}"
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
  infomation = "{{ index .QueryResults `cw_logs` | to_vertical }}"
}
```

