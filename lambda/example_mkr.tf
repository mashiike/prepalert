resource "mackerel_service" "prepalert" {
  name = "prepalert"
}

resource "mackerel_monitor" "manual_trigger" {
  name = "[prepalert] manual trigger metrics"
  memo = "managed by terraform: repo is github.com/mashiike/prepalert"
  service_metric {
    service  = mackerel_service.prepalert.name
    metric   = "manual.trigger.value"
    operator = ">"
    duration = 1
    warning  = 0
  }
}

resource "mackerel_channel" "lambda_function_url" {
  name = "[prepalert] lambda function url"

  webhook {
    url = aws_lambda_function_url.prepalert.function_url

    events = [
      "alert",
      "alertGroup",
    ]
  }
}

resource "mackerel_notification_group" "prepalert" {
  name = "prepalert"

  child_channel_ids = [
    mackerel_channel.lambda_function_url.id,
  ]

  service {
    name = mackerel_service.prepalert.name
  }
  monitor {
    id           = mackerel_monitor.manual_trigger.id
    skip_default = true
  }
}
