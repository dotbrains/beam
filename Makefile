BINARY := beam
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test lint install clean vet

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1

vet:
	go vet ./...

lint:
	golangci-lint run

clean:
	rm -f $(BINARY) coverage.out

cover: test
	go tool cover -html=coverage.out

.DEFAULT_GOAL := build
