export AWS_ACCOUNT_ID := $(shell aws sts get-caller-identity --query 'Account' --output text)
.PHONY: clean deploy-all

bootstrap: ../cmd/prepalert/*.go ../*.go ../go.*
	GOARCH=amd64 GOOS=linux go build -o bootstrap ../cmd/prepalert/*.go

clean:
	rm -f bootstrap

deploy-all: deploy/http deploy/worker

deploy/%: bootstrap
	PREPALERT_MODE=$* lambroll --log-level debug deploy
	$(MAKE) logs/$*

logs/%:
	PREPALERT_MODE=$* lambroll logs --follow --format=short

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
