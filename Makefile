.PHONY: fmt fmt-check lint test build check

fmt:       ## apply formatters (gofumpt & goimports)
	golangci-lint fmt

fmt-check: ## verify formatting without writing
	golangci-lint fmt --diff

lint:      ## static analysis (staticcheck, govet, errcheck, etc.)
	golangci-lint run

test:      ## unit tests
	go test ./...

build:     ## build the jard binary
	go build -o jard .

check: fmt-check lint test ## run every check
