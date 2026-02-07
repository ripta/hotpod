.PHONY: build test lint docker-build k8s-validate quick pre-commit all help
.DEFAULT_GOAL := help

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.version=$(VERSION)

build: ## Build binary to bin/hotpod
	@go build -ldflags "$(LDFLAGS)" -o bin/hotpod ./cmd/hotpod

test: ## Run tests with coverage
	@go test -cover ./...

lint: ## Run go vet
	@go vet ./...

docker-build: ## Build Docker image (hotpod:dev)
	@docker build -t hotpod:dev .

k8s-validate: ## Validate manifests (dry-run)
	@kubectl apply --dry-run=client -k manifests/base/

quick: build lint ## build + lint
pre-commit: build test lint ## build + test + lint
all: build test lint docker-build k8s-validate ## Run all targets

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'
