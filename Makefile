BINARY   := tf
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -X main.version=$(VERSION)

.PHONY: all build install test vet fmt lint check clean

all: check build

## build: compile binary to ./tf
build: fmt test
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

## install: install binary to GOPATH/bin
install: clean fmt test
	go install -ldflags "$(LDFLAGS)" .

## test: run all tests with race detector and coverage
test:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## vet: run go vet
vet:
	go vet ./...

## fmt: format source files (reports diff, does not modify)
fmt:
	@gofmt -l . | grep -q . && echo "gofmt: unformatted files found (run: gofmt -w .)" && exit 1 || echo "gofmt: ok"

## lint: run staticcheck (go install honnef.co/go/tools/cmd/staticcheck@latest)
lint:
	@which staticcheck > /dev/null 2>&1 || (echo "staticcheck not found; skipping (go install honnef.co/go/tools/cmd/staticcheck@latest)" && exit 0)
	staticcheck ./...

## check: run full local pipeline (fmt, vet, lint, test)
check: fmt vet lint test

## clean: remove build artifacts
clean:
	rm -f $(BINARY) coverage.out

## help: show this help
help:
	@grep -E '^## ' Makefile | sed 's/^## //'
