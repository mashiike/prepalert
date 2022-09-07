
resource "aws_iam_role" "prepalert" {
  name = "prepalert"

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
    ]
    resources = [aws_sqs_queue.prepalert.arn]
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
  name                       = "dev-prepalert"
  message_retention_seconds  = 86400
  visibility_timeout_seconds = 900
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.prepalert-dlq.arn
    maxReceiveCount     = 3
  })
}

resource "aws_sqs_queue" "prepalert-dlq" {
  name                      = "dev-prepalert-dlq"
  message_retention_seconds = 345600
}

data "aws_lambda_alias" "prepalert_webhook" {
  function_name = "repalert-webhook"
  name          = "current"
}

data "aws_lambda_alias" "prepalert_worker" {
  function_name = "prepalert-worker"
  name          = "current"
}

resource "aws_lambda_function_url" "prepalert_webhook" {
  function_name      = data.aws_lambda_alias.prepalert_webhook.function_name
  qualifier          = data.aws_lambda_alias.prepalert_webhook.name
  authorization_type = "NONE"

  cors {
    allow_credentials = true
    allow_origins     = ["*"]
    allow_methods     = ["POST"]
    expose_headers    = ["keep-alive", "date"]
    max_age           = 0
  }
}

resource "aws_lambda_event_source_mapping" "prepalert_worker_invoke_from_sqs" {
  batch_size       = 1
  event_source_arn = aws_sqs_queue.prepalert.arn
  enabled          = true
  function_name    = data.aws_lambda_alias.prepalert_worker.arn
}
