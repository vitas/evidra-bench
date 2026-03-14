BINARY := infra-bench
VERSION ?= dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test test-race fmt lint tidy clean smoke

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

smoke: build
	bash tests/smoke/run_local_smoke.sh
