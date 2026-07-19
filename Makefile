.PHONY: fmt fmt-check lint test build check release-check

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

fmt:       ## apply formatters (gofumpt & goimports)
	golangci-lint fmt

fmt-check: ## verify formatting without writing
	golangci-lint fmt --diff

lint:      ## static analysis (staticcheck, govet, errcheck, etc.)
	golangci-lint run

test:      ## unit tests
	go test ./...

build:     ## build the jard binary
	go build -ldflags "-X main.version=$(VERSION)" -o jard .

check: fmt-check lint test ## run every check

release-check: ## dry-run the release build locally (no publish)
	goreleaser release --snapshot --clean --skip=publish
