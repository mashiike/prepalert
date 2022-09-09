
resource "aws_iam_role" "prepalert" {
  name = "prepalert_lambda"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Sid    = ""
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_policy" "prepalert" {
  name   = "prepalert"
  path   = "/"
  policy = data.aws_iam_policy_document.prepalert.json
}

resource "aws_iam_role_policy_attachment" "prepalert" {
  role       = aws_iam_role.prepalert.name
  policy_arn = aws_iam_policy.prepalert.arn
}

data "aws_iam_policy_document" "prepalert" {
  statement {
    actions = [
      "sqs:DeleteMessage",
      "sqs:GetQueueUrl",
      "sqs:ChangeMessageVisibility",
      "sqs:ReceiveMessage",
      "sqs:SendMessage",
      "sqs:GetQueueAttributes",
    ]
    resources = [aws_sqs_queue.prepalert.arn]
  }
  statement {
    actions = [
      "ssm:GetParameter*",
      "ssm:DescribeParameters",
      "ssm:List*",
    ]
    resources = ["*"]
  }
    statement {
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:List*",
    ]
    resources = ["*"]
  }
  statement {
    actions = [
      "logs:GetLog*",
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    resources = ["*"]
  }
}

resource "aws_sqs_queue" "prepalert" {
  name                       = "prepalert"
  message_retention_seconds  = 86400
  visibility_timeout_seconds = 900
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.prepalert-dlq.arn
    maxReceiveCount     = 3
  })
}

resource "aws_sqs_queue" "prepalert-dlq" {
  name                      = "prepalert-dlq"
  message_retention_seconds = 345600
}

data "archive_file" "prepalert_dummy" {
  type        = "zip"
  output_path = "${path.module}/prepalert_dummy.zip"
  source {
    content  = "prepalert_dummy"
    filename = "bootstrap"
  }
  depends_on = [
    null_resource.prepalert_dummy
  ]
}

resource "null_resource" "prepalert_dummy" {}

resource "aws_lambda_function" "prepalert_http" {
  lifecycle {
    ignore_changes = all
  }

  function_name = "prepalert-http"
  role          = aws_iam_role.prepalert.arn

  handler  = "bootstrap"
  runtime  = "provided.al2"
  filename = data.archive_file.prepalert_dummy.output_path
}

resource "aws_lambda_alias" "prepalert_http" {
  lifecycle {
    ignore_changes = all
  }
  name             = "current"
  function_name    = aws_lambda_function.prepalert_http.arn
  function_version = aws_lambda_function.prepalert_http.version
}

resource "aws_lambda_function" "prepalert_worker" {
  lifecycle {
    ignore_changes = all
  }

  function_name = "prepalert-worker"
  role          = aws_iam_role.prepalert.arn

  handler  = "bootstrap"
  runtime  = "provided.al2"
  filename = data.archive_file.prepalert_dummy.output_path
}

resource "aws_lambda_alias" "prepalert_worker" {
  lifecycle {
    ignore_changes = all
  }
  name             = "current"
  function_name    = aws_lambda_function.prepalert_worker.arn
  function_version = aws_lambda_function.prepalert_worker.version
}

resource "aws_lambda_function_url" "prepalert_http" {
  function_name      = aws_lambda_alias.prepalert_http.function_name
  qualifier          = aws_lambda_alias.prepalert_http.name
  authorization_type = "NONE"

  cors {
    allow_credentials = true
    allow_origins     = ["*"]
    allow_methods     = ["POST", "GET"]
    expose_headers    = ["keep-alive", "date"]
    max_age           = 0
  }
}

resource "aws_lambda_event_source_mapping" "prepalert_worker_invoke_from_sqs" {
  batch_size       = 1
  event_source_arn = aws_sqs_queue.prepalert.arn
  enabled          = true
  function_name    = aws_lambda_alias.prepalert_worker.arn
}

resource "aws_ssm_parameter" "mackerel_apikey" {
  name        = "/prepalert/MACKEREL_APIKEY"
  description = "Mackerel API Key for prepalert"
  type        = "SecureString"
  value       = local.mackerel_apikey
}
