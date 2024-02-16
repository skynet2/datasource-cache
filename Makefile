.PHONY: lint
lint:
	golangci-lint run

.PHONY: generate
generate:
	go generate ./...