BINARY := bench-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION ?= $(shell awk '/^go / { print $$2 }' go.mod)
GOVULNCHECK_TOOLCHAIN ?= go$(GO_VERSION)
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)"

.PHONY: build db-import test test-race fmt lint vuln tidy clean smoke public-smoke public-smoke-test private-review-smoke private-review-smoke-test catalog core-report ui-install ui-dev ui-build ui-docker docker-bench docker-contract docker-one-command-smoke core-contract-kind core-contract-k3d

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
	GOTOOLCHAIN=$(GOVULNCHECK_TOOLCHAIN) go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/ runs/

smoke: build
	bash tests/smoke/run_local_smoke.sh

# kubernetes-core@1 exact-verdict contract matrix (three control
# evaluations per repetition on one disposable cluster each). Requires a
# sandbox image carrying kubectl; build it once with `make docker-contract`
# or the printed command.
core-contract-kind:
	bash tests/contract/run_kubernetes_core_contract.sh kind

core-contract-k3d:
	bash tests/contract/run_kubernetes_core_contract.sh k3d

public-smoke:
	bash tests/smoke/run_public_api_smoke.sh

public-smoke-test:
	bash tests/smoke/test_public_api_smoke.sh

private-review-smoke:
	bash tests/smoke/run_private_review_smoke.sh

private-review-smoke-test:
	bash tests/smoke/test_private_review_smoke.sh

ui-install:
	cd ui && npm ci

ui-dev:
	cd ui && npm run dev

catalog:
	go run scripts/generate-catalog.go

# Public core-pack report: regenerate ui/src/data/coreReport.ts from a run
# directory produced by `bench-cli test --suite kubernetes-core@1`.
# Override CORE_* to publish a different run or target.
CORE_REPORT_DIR ?= runs/public-2026-09-13-core-gemini-flash
CORE_REPORT_ID ?= kubernetes-core-v1-2026-09
CORE_REPORT_LABEL ?= Gemini 2.5 Flash (free tier)
CORE_REPORT_MODEL ?= gemini-2.5-flash
CORE_REPORT_PROVIDER ?= openai-compatible
CORE_REPORT_ENDPOINT ?= https://generativelanguage.googleapis.com/v1beta/openai/
CORE_REPORT_NOTES ?= Single free-tier run: gemini-2.5-flash through the Google AI Studio OpenAI-compatible endpoint, kind provider, one repetition per case. Nothing was re-scored or filtered for publication.

core-report:
	go run scripts/generate-core-report.go \
		-dir "$(CORE_REPORT_DIR)" \
		-out ui/src/data/coreReport.ts \
		-report-id "$(CORE_REPORT_ID)" \
		-label "$(CORE_REPORT_LABEL)" \
		-model "$(CORE_REPORT_MODEL)" \
		-provider "$(CORE_REPORT_PROVIDER)" \
		-endpoint "$(CORE_REPORT_ENDPOINT)" \
		-notes "$(CORE_REPORT_NOTES)"

ui-build: catalog
	cd ui && npm ci && npm run build

ui-docker:
	docker build -t ghcr.io/vitas/evidra-bench-ui:latest ui/

docker-bench:
	docker build -f Dockerfile.bench \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t ghcr.io/vitas/evidra-bench:latest \
		-t ghcr.io/vitas/evidra-bench:$(VERSION) \
		-t ghcr.io/vitas/evidra-bench-cli:latest \
		-t ghcr.io/vitas/evidra-bench-cli:$(VERSION) \
		.

docker-contract:
	bash tests/test_docker_runner_contract.sh

docker-one-command-smoke:
	bash tests/smoke/run_docker_one_command_smoke.sh
