BINARY := infra-bench
VERSION ?= dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test fmt lint tidy clean

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/infra-bench

test:
	go test ./... -v -count=1

test-race:
	go test -race ./... -count=1

fmt:
	gofmt -w .

lint:
	golangci-lint run

tidy:
	go mod tidy

clean:
	rm -rf bin/ runs/
