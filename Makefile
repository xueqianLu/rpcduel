BINARY      ?= rpcduel
PKG         := ./...
GO          ?= go
GOFLAGS     ?=
LDFLAGS     ?= -s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: all build install test race cover vet lint tidy fmt clean release-snapshot docker ci manpages completions

all: build

build:
	$(GO) build $(GOFLAGS) -ldflags='$(LDFLAGS)' -o bin/$(BINARY) .

install:
	$(GO) install $(GOFLAGS) -ldflags='$(LDFLAGS)' .

manpages: build
	@mkdir -p dist/man
	./bin/$(BINARY) man --dir dist/man

completions: build
	@mkdir -p dist/completions
	./bin/$(BINARY) completion bash       > dist/completions/$(BINARY).bash
	./bin/$(BINARY) completion zsh        > dist/completions/$(BINARY).zsh
	./bin/$(BINARY) completion fish       > dist/completions/$(BINARY).fish
	./bin/$(BINARY) completion powershell > dist/completions/$(BINARY).ps1

test:
	$(GO) test $(GOFLAGS) -count=1 $(PKG)

race:
	$(GO) test $(GOFLAGS) -race -count=1 $(PKG)

cover:
	$(GO) test $(GOFLAGS) -race -covermode=atomic -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out | tail -n 1

vet:
	$(GO) vet $(PKG)

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not installed: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt $(PKG)

clean:
	rm -rf bin/ coverage.out dist/

release-snapshot:
	@command -v goreleaser >/dev/null 2>&1 || { \
		echo "goreleaser not installed: https://goreleaser.com/install/"; exit 1; }
	goreleaser release --snapshot --clean

docker:
	docker build -t $(BINARY):dev .

ci: vet lint race
	@echo "ci targets passed"
