.PHONY: build test lint bench docker-build k8s-validate k6-configmaps quick pre-commit all help
.DEFAULT_GOAL := help

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.version=$(VERSION)

# pprof-overhead benchmark knobs (override on the command line, e.g.
# `make bench BENCH_COUNT=10 BENCH_OUT=/tmp/bench.txt`).
BENCH_TIME  ?= 1s
BENCH_COUNT ?= 6
BENCH_OUT   ?= benchmarks/pprof-overhead.txt

build: ## Build binary to bin/hotpod
	@go build -ldflags "$(LDFLAGS)" -o bin/hotpod ./cmd/hotpod

test: ## Run tests with coverage
	@go test -cover ./...

lint: ## Run go vet
	@go vet ./...

bench: ## Measure pprof overhead (CPU + queue); writes results to $(BENCH_OUT)
	@mkdir -p $(dir $(BENCH_OUT))
	@go test -run '^$$' -bench 'ProfileOverhead' -benchtime=$(BENCH_TIME) -count=$(BENCH_COUNT) \
		./internal/handlers/ ./internal/queue/ | tee $(BENCH_OUT)

docker-build: ## Build Docker image (hotpod:dev)
	@docker build -t hotpod:dev .

k8s-validate: ## Validate manifests (dry-run)
	@kubectl apply --dry-run=client -k manifests/base/

k6-configmaps: ## Generate k6 operator ConfigMaps from scenario scripts
	@scenarios/k6-operator/build-configmaps.sh > scenarios/k6-operator/configmaps.yaml

quick: build lint ## build + lint
pre-commit: build test lint ## build + test + lint
all: build test lint docker-build k8s-validate ## Run all targets

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-17s %s\n", $$1, $$2}'
