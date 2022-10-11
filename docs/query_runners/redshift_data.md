## Feature: Redshift data Query runner

sample configuration

```hcl
prepalert {
    required_version = ">=v0.7.0"
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
WHERE status BETWEEN 500 AND 599
    AND "time" BETWEEN 
        '${strftime("%Y-%m-%dT%H:%M:%SZ","UTC", runtime.event.alert.opened_at)}'::timestamp - interval '15 minutes'
        AND '${strftime("%Y-%m-%dT%H:%M:%SZ","UTC", runtime.event.alert.closed_at)}'::timestamp
LIMIT 200
EOQ
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
${runtime.query_result.alb_target_5xx_info.table}
EOT
}
```

### query_runner block

Specify the target Redshift to query.
There are several ways to specify.

#### with secrets_arn 

```hcl
query_runner "redshift_data" "default" {
    secrets_arn = "arn:aws:secretsmanager:ap-northeast-1:xxxxxxxxxxxx:secret:test-1O5wUG"
}
```

#### with cluster_identifier, db_user, database

only provisioned cluster

```hcl
query_runner "redshift_data" "default" {
    cluster_identifier = "warehouse"
    database           = "dev"
    db_user            = "admin"
}
```

#### with workgroup_name, database

only serverless workgroup

```hcl
query_runner "redshift_data" "default" {
    workgroup_name = "default"
    database       = "dev"
}
```

### query block

```hcl
query "alb_target_5xx_info" {
    runner = query_runner.redshift_data.default
    sql = <<EOQ
SELECT *
FROM access_logs
WHERE status BETWEEN 500 AND 599
    AND "time" BETWEEN 
        '${strftime("%Y-%m-%dT%H:%M:%SZ","UTC", runtime.event.alert.opened_at)}'::timestamp - interval '15 minutes'
        AND '${strftime("%Y-%m-%dT%H:%M:%SZ","UTC", runtime.event.alert.closed_at)}'::timestamp
LIMIT 200
EOQ
}
```

only sql attribute.


