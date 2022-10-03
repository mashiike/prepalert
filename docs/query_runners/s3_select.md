## Feature: S3 select Query runner

sample configuration

```hcl
prepalert {
    required_version = ">=v0.2.0"
    sqs_queue_name   = "prepalert"
    service          = "prod"
}

query_runner "s3_select" "default" {
  region = "ap-northeast-1"
}

query "alb_5xx_logs" {
  runner            = query_runner.s3_select.default
  bucket_name       = "your-bucket"
  object_key_prefix = "alb/AWSLogs/0123456789012/elasticloadbalancing/ap-northeast-1/{{ .Alert.OpenedAt | to_time | strftime `%Y/%m/%d` }}/"
  compression_type  = "GZIP"
  csv {
    field_delimiter  = " "
    record_delimiter = "\n"
  }
  expression = file("get_alb_5xx_log.sql")
}

rule "alb_target_5xx" {
    alert {
        monitor_name = "ALB Target 5xx"
    }

    queries = [
        query.alb_target_5xx_info,
    ]

    infomation = <<EOT
5xx info:
{{ index .QueryResults `alb_target_5xx_info` | to_table }}
EOT
}
```

### query runner block

aws region only: Required

### query block

When querying uncompressed json lines, the following is used

```hcl
query "logs" {
  runner            = query_runner.s3_select.default
  bucket_name       = "your-bucket"
  object_key_prefix = "application-logs/{{ .Alert.OpenedAt | to_time | strftime `%Y/%m/%d` }}/"
  compression_type  = "NONE"
  json {
    type = "LINES"
  }
  expression = file("logs.sql")
}
```


in the case of Parquet

```hcl
query "logs" {
  runner            = query_runner.s3_select.default
  bucket_name       = "your-bucket"
  object_key_prefix = "application-logs/{{ .Alert.OpenedAt | to_time | strftime `%Y/%m/%d` }}/"
  compression_type  = "NONE"
  parquet {}
  expression = file("logs.sql")
}
```