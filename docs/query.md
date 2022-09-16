## Feature: query and query_runner

In the simplest setting, there were only fixed messages.
However, some alerts may want to be accompanied by logs or other metric information.
This is where `query` and `query_runner` come in.

The query_runner is a description of the means of obtaining information.
The query is a description of the actual information to be obtained.

There are different types of query_runners, and the attributes specified for queries differ for each type of query_runner.
See [docs/query_runners](query_runners/) for detailed settings for each query_runner.

The query and query_runner settings are generally as follows

```hcl
prepalert {
    required_version = ">=v0.2.0"
    sqs_queue_name   = "prepalert"
    service          = "prod"
}

query_runner "<query_runner_type>" "<query_runner_name>" {
    // Different settings for different query runners...
}

query "<query_name>" {
    runner = query_runner.<query_runner_type>.<query_runner_name>
    // Different settings for different query runners...
}

rule "simple" {
    alert {
        any = true
    }
    queries = [
        query.<query_name>,
    ]

    infomation = <<EOF
query_result:
{{ index .QueryResults `<query_name>` | to_table }}
EOF
}
```

The queries attribute of a rule lists the queries to be executed.
Then, using the go template notation, the results of the query can be referenced in the infomation attribute.

### template function
The results of the referenced query are output using a stringing function such as to_table.

#### to_table 

The following standard table is provided

```
+--------+-------+--------+
| STATUS | COUNT |    P99 |
+--------+-------+--------+
|    5xx |   300 |  0.788 |
|    4xx |  2000 | 0.5022 |
+--------+---- --+--------+
```

#### to_vertical 

The following mysql \G option like table is provided

```
********* 1. row *********
status: 5xx
count: 300
p99: 0.788
********* 2. row *********
status: 4xx
count: 2000
p99: 0.5022
```


#### to_json 

output as json lines

```
{"status":"5xx", "count":"300", "p99":"0.788"}
{"status":"4xx", "count":"2000", "p99":"0.5022"}
```

### How to check, query running

on local 
```shell
$ prepalert exec <alert_id>
```

Locally, simulations can be performed based on past alerts.
Does the RULE match this way? Does the QUERY work? can be checked.
