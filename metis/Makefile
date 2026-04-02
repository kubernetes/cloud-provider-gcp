# Makefile for metis component

# Variables for local development
IMAGE_REGISTRY ?= gcr.io/$(USER)-gke-dev
IMAGE_NAME ?= metis
TAG ?= $(shell git describe --tags --match "metis-v*" --always --dirty | sed 's|.*/||')

# Get version metadata from git for version injection
GIT_VERSION ?= $(shell git describe --tags --match "metis-v*" --always --dirty)
GIT_COMMIT  ?= $(shell git rev-parse HEAD)
BUILD_DATE  ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Standard K8s versioning - injected via linker flags at build time.
VERSION_LDFLAGS := \
  -X 'k8s.io/component-base/version.gitVersion=$(GIT_VERSION)' \
  -X 'k8s.io/component-base/version.gitCommit=$(GIT_COMMIT)' \
  -X 'k8s.io/component-base/version.buildDate=$(BUILD_DATE)'

# Platforms for multi-arch container builds
PLATFORMS ?= linux/amd64,linux/arm64

##@ Development

.PHONY: tidy
tidy: ## Tidy the Go module dependencies.
	go mod tidy

.PHONY: fmt
fmt: ## Format the Go source code.
	go fmt ./...

.PHONY: vet
vet: ## Run Go vet to check for potential issues.
	go vet ./...

.PHONY: build
build: ## Build the Metis binary locally using workspace dependencies.
	@mkdir -p bin
	CGO_ENABLED=1 go build -a -o bin/metis -ldflags "$(VERSION_LDFLAGS)" ./cmd

.PHONY: test
test: ## Run the Go tests.
	go test ./...

.PHONY: proto
proto: ## Generate code from proto files.
	../tools/update-proto.sh

.PHONY: all
all: clean tidy fmt vet test build ## Run all standard development targets.

.PHONY: clean
clean: ## Clean up build artifacts.
	rm -rf bin
	go clean

##@ Docker

.PHONY: push-image
push-image: ## Build and push the multi-arch image independently.
	@echo "Building and Pushing multi-arch image: $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(TAG)..."
	docker buildx build --platform $(PLATFORMS) \
		--build-arg GIT_VERSION=$(GIT_VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(TAG) --push .

##@ Help

.PHONY: help
help: ## Display this help message.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
