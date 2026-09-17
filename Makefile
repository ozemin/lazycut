GO ?= go
GOLANGCI_LINT ?= golangci-lint
GOVULNCHECK ?= $(GO) run golang.org/x/vuln/cmd/govulncheck@latest

.PHONY: build test test-race lint fmt fmt-check tidy vuln check clean

build:
	$(GO) build -o lazycut .

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

lint:
	$(GOLANGCI_LINT) run

fmt:
	$(GOLANGCI_LINT) fmt

fmt-check:
	$(GOLANGCI_LINT) fmt --diff

tidy:
	$(GO) mod tidy

vuln:
	$(GOVULNCHECK) ./...

check: fmt-check lint test-race vuln

clean:
	rm -f lazycut
