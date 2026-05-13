BINARY := bench-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)"

.PHONY: build db-import test test-race fmt lint vuln tidy clean smoke public-smoke public-smoke-test catalog ui-install ui-dev ui-build ui-docker docker-bench

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/bench-cli

db-import: build
	bin/$(BINARY) db import --runs-dir runs

test:
	go test ./... -v -count=1

test-race:
	go test -race ./... -count=1

fmt:
	gofmt -w .

lint:
	golangci-lint run

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/ runs/

smoke: build
	bash tests/smoke/run_local_smoke.sh

public-smoke:
	bash tests/smoke/run_public_api_smoke.sh

public-smoke-test:
	bash tests/smoke/test_public_api_smoke.sh

ui-install:
	cd ui && npm ci

ui-dev:
	cd ui && npm run dev

catalog:
	go run scripts/generate-catalog.go

ui-build: catalog
	cd ui && npm ci && npm run build

ui-docker:
	docker build -t ghcr.io/vitas/bench-ui:latest ui/

docker-bench:
	docker build -f Dockerfile.bench \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t ghcr.io/vitas/bench-cli:latest \
		-t ghcr.io/vitas/bench-cli:$(VERSION) \
		.
