## Feature: Basic Auth

When using a Lambda Function URL to receive a webhook from Mackerel, the authentication of the Lambda Function URL should be NONE.
However, this means that anyone can POST the URL if it is compromised.
Therefore, there is a function to enable Basic Authentication.

```hcl
prepalert {
    required_version = ">=v0.2.0"
    sqs_queue_name   = "prepalert"
    service          = "prod"

    auth {
        client_id     = "prepalert"
        client_secret = "<your basic pass>"
    }
}

rule "simple" {
    alert {
        any = true
    }
    information = <<EOF
How do you respond to alerts?
Describe information about your alert response here.
EOF
}
```

If set up in this way, basic authentication will be applied to the webhook.
Suppose the original Lambda Function URL is `https://<function_url_id>.lambda-url.ap-northeast-1.on.aws/`.

When registering with Mackerel, you will register the following URL.
`https://prepalert:<your basic pass>@<function_url_id>.lambda-url.ap-northeast-1.on.aws/`
