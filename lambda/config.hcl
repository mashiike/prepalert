prepalert {
  required_version = ">=v0.3.1"
  sqs_queue_name   = "prepalert"
  service          = "prepalert-dev"
}

rule "simple" {
  alert {
    any = true
  }
  infomation = "How do you respond to alerts?"
}
