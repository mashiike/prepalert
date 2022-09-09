export AWS_ACCOUNT_ID := $(shell aws sts get-caller-identity --query 'Account' --output text)
.PHONY: clean deploy-all

bootstrap: ../cmd/prepalert/*.go ../*.go ../go.*
	GOARCH=amd64 GOOS=linux go build -o bootstrap ../cmd/prepalert/*.go

clean:
	rm -f bootstrap
	rm -f *_dummy.zip

deploy-all: deploy/http deploy/worker

deploy/%: bootstrap
	PREPALERT_MODE=$* lambroll --log-level debug deploy --tfstate .terraform/terraform.tfstate

logs/%:
	PREPALERT_MODE=$* lambroll logs --follow --format=short --tfstate .terraform/terraform.tfstate

.PHONY: terraform/init
terraform/init:
	terraform providers lock \
		-platform=darwin_amd64 \
		-platform=darwin_arm64 \
		-platform=linux_amd64 \
		-platform=linux_arm64 \
		-platform=windows_amd64
	terraform init


.PHONY: terraform/plan
terraform/plan:
	terraform plan


.PHONY: terraform/apply
terraform/apply:
	terraform apply

.PHONY: send-event
send-event:
	cat event.json | curl -d @- -H "Content-Type: application/json" $(shell tfstate-lookup --state .terraform/terraform.tfstate aws_lambda_function_url.prepalert_http.function_url)
	$(MAKE) logs/worker
