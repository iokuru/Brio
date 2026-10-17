.PHONY: all build test lint fmt

all: lint test build

build:
	go build ./cmd/brio

test:
	go test -race -cover ./...

lint:
	go vet ./...

fmt:
	go fmt ./...
