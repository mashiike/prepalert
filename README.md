# prepalert

![Latest GitHub release](https://img.shields.io/github/release/mashiike/prepalert.svg)
![Github Actions test](https://github.com/mashiike/prepalert/workflows/Test/badge.svg?branch=main)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/mashiike/prepalert/blob/master/LICENSE)

Toil reduction tool to prepare before responding to Mackerel alerts

`preplert` consists of two parts: a webhook server that receives Mackerel webhooks and sends the payload to Amazon SQS, and a worker that queries various data based on the webhooks and pastes information for alert response as a GraphAnnotation.


## Install 

#### Homebrew (macOS and Linux)

```console
$ brew install mashiike/tap/prepalert
```

### Binary packages

[Releases](https://github.com/mashiike/prepalert/releases)

## QuickStart 

Set your Mackerel API key to the environment variable `MACKEREL_APIKEY`.  
and Execute the following command:

```shell
$ prepalert init 
```

Or the following command:
```shell 
$ prepalert --coinfig <output config path> init
```

## Usage 

```
NAME:
   prepalert - A webhook server for prepare alert memo

USAGE:
   prepalert -config <config file> [command options]

VERSION:
   current

COMMANDS:
   init     create inital config
   run      run server (default command)
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --address value                    run address (default: ":8080") [$PREPALERT_ADDRESS]
   --batch-size value                 run local sqs batch size (default: 1) [$PREPALERT_BATCH_SIZE]
   --config value, -c value           config path (default: ".") [$CONFIG, $PREPALERT_CONFIG]
   --help, -h                         show help (default: false)
   --log-level value                  output log-level (default: "info") [$PREPALERT_LOG_LEVEL]
   --mackerel-apikey value, -k value  for access mackerel API (default: *********) [$MACKEREL_APIKEY, $PREPALERT_MACKEREL_APIKEY]
   --mode value                       run mode (default: "http") [$PREPALERT_MODE]
   --prefix value                     run server prefix (default: "/") [$PREPALERT_PREFIX]
   --version, -v                      print the version (default: false)
```

If the command is omitted, the run command is executed.

## Configurations

Configuration file is HCL (HashiCorp Configuration Language) format. `prepalert init` can generate a initial configuration file.

The most small configuration file is as follows:
```hcl
prepalert {
    required_version = ">=v0.2.0"
    sqs_queue_name   = "prepalert"
    service          = "prod"
}

rule "any_alert" {
    alert {
        any = true
    }

    infomation = <<EOF
How do you respond to alerts?
Describe information about your alert response here.
(This area can use Go's template notation.)
EOF
}
```

## Usage with AWS Lambda (serverless)

prepalert works with AWS Lambda and Amazon SQS.

Lambda Function requires a webhook and a worker


```mermaid
sequenceDiagram
  autonumber
  Mackerel->>+http lambda function : POST /
  http lambda function ->>+Amazon SQS: SendMessage
  Amazon SQS-->- http lambda function: 200 Ok
  http lambda function-->- Mackerel: 200 Ok
  Amazon SQS ->>+ worker lambda function: trigger by AWS Lambda
  worker lambda function ->>+ Data Source: query
  Data Source -->- worker lambda function: query results
  worker lambda function  ->>+ Mackerel: Create Graph Annotation
  Mackerel-->- worker lambda function : 200 Ok
  worker lambda function ->>-  Amazon SQS: Success Delete message
```


Let's solidify the Lambda package with the following zip arcive (runtime `provided.al2`)

```
lambda.zip
├── bootstrap    # build binary
└── config.hcl   # configuration file
```

A related document is [https://docs.aws.amazon.com/lambda/latest/dg/runtimes-custom.html](https://docs.aws.amazon.com/lambda/latest/dg/runtimes-custom.html)

for example.

deploy two lambda functions, prepalert-http and prepalert-worker in [lambda directory](lambda/)  
The example of lambda directory uses [lambroll](https://github.com/fujiwara/lambroll) for deployment.

For more information on the infrastructure around lambda functions, please refer to [example.tf](lambda/example.tf).

## LICENSE

MIT License

Copyright (c) 2022 IKEDA Masashi
