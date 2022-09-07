
.PHONY: clean

bootstrap: ../cmd/prepalert/*.go ../*.go ../go.*
	GOARCH=amd64 GOOS=linux go build -o bootstrap ../cmd/prepalert/*.go

clean:
	rm -f bootstrap

deploy/%: bootstrap
	PREPALERT_MODE=$* lambroll --log-level debug deploy
	$(MAKE) logs/$*

logs/%:
	PREPALERT_MODE=$* lambroll logs --follow --format=short
