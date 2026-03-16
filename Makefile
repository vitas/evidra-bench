BINARY := infra-bench
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)"

.PHONY: build build-api build-api-ui ui db-import test test-race fmt lint tidy clean smoke docker

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/infra-bench

build-api:
	go build $(LDFLAGS) -o bin/bench-api ./cmd/bench-api

ui:
	cd ui && npm ci && npm run build

build-api-ui: ui
	go build -tags embed_ui $(LDFLAGS) -o bin/bench-api ./cmd/bench-api

db-import: build
	bin/$(BINARY) db import --runs-dir runs

docker:
	docker build -f Dockerfile.api --build-context parent=../evidra-benchmark -t bench-api:local .

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
