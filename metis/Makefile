# Makefile for metis component

# Variables for local development
IMAGE_REGISTRY ?= gcr.io/$(USER)-gke-dev
IMAGE_NAME ?= metis
TAG ?= $(shell git describe --tags --always --dirty | sed 's|.*/||')

# Get version metadata from git
GIT_VERSION ?= $(shell git describe --tags --always --dirty)
GIT_COMMIT  ?= $(shell git rev-parse HEAD)
BUILD_DATE  ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Standard K8s versioning - injected via linker flags
VERSION_LDFLAGS := \
  -X 'k8s.io/component-base/version.gitVersion=$(GIT_VERSION)' \
  -X 'k8s.io/component-base/version.gitCommit=$(GIT_COMMIT)' \
  -X 'k8s.io/component-base/version.buildDate=$(BUILD_DATE)'

##@ Development

.PHONY: fmt
fmt: ## Format the Go source code.
	go fmt ./...

.PHONY: vet
vet: ## Run Go vet to check for potential issues.
	go vet ./...

.PHONY: vendor
vendor: ## Tidy and vendor the Go module dependencies.
	go mod tidy
	go mod vendor

.PHONY: build
build: ## Build the Metis binary locally.
	@mkdir -p bin
	CGO_ENABLED=1 go build -a -o bin/metis -ldflags "$(VERSION_LDFLAGS)" ./cmd

.PHONY: test
test: ## Run the Go tests.
	go test ./...

.PHONY: lint
lint: vet ## Run the linters.

.PHONY: all
all: fmt lint test build ## Run all standard development targets.

.PHONY: clean
clean: ## Clean up build artifacts.
	rm -rf bin
	go clean

##@ Docker

.PHONY: push-image
push-image: ## Build and push the multi-arch image using the repo's shared tool.
	@echo "Building and Pushing Metis image..."
	REGISTRY=$(IMAGE_REGISTRY) IMAGE=$(IMAGE_NAME) TAG=$(TAG) ../tools/push-images metis

##@ Help

.PHONY: help
help: ## Display this help message.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
