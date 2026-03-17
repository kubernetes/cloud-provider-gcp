# Makefile for metis component

# Variables for local development
IMAGE_REGISTRY ?= gcr.io/$(USER)-gke-dev
IMAGE_NAME ?= metis
TAG ?= $(shell git describe --tags --always --dirty | sed 's|.*/||')
BUILDER_NAME := metis-builder

# Variables for Louhi
LOUHI_REGISTRY ?= $(IMAGE_REGISTRY)
LOUHI_IMAGE_NAME ?= $(IMAGE_NAME)
LOUHI_TAG ?= $(TAG)

# Platforms for multi-arch build
PLATFORMS ?= linux/amd64,linux/arm64

GO_MODULE := k8s.io/metis
VERSION_LDFLAGS := -X '$(GO_MODULE)/pkg/version.Version=$(TAG)'

# Full image paths
DEV_IMAGE := $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(TAG)
LOUHI_IMAGE := $(LOUHI_REGISTRY)/$(LOUHI_IMAGE_NAME):$(LOUHI_TAG)

# Enable Docker BuildKit
export DOCKER_BUILDKIT=1
export DOCKER_CLI_EXPERIMENTAL=enabled

##@ Development

.PHONY: fmt
fmt: ## Format the Go source code.
	go fmt ./...

.PHONY: vet
vet: ## Run Go vet to check for potential issues.
	go vet ./...

.PHONY: vendor
vendor: ## Tidy and vendor the Go module dependencies.
	@GOPRIVATE=*.googlesource.com,*.git.corp.google.com go mod tidy
	@GOPRIVATE=*.googlesource.com,*.git.corp.google.com go mod vendor

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

.PHONY: ensure-builder
ensure-builder:
	@docker buildx inspect $(BUILDER_NAME) > /dev/null 2>&1 || docker buildx create --name $(BUILDER_NAME) --use

.PHONY: docker-manifest
docker-manifest: ensure-builder ## Build and push multi-arch manifest using buildx.
	@echo "Building and Pushing multi-arch image: $(DEV_IMAGE)..."
	docker buildx build --platform $(PLATFORMS) \
		--builder $(BUILDER_NAME) \
		--build-arg VERSION_LDFLAGS="$(VERSION_LDFLAGS)" \
		-t $(DEV_IMAGE) --push .

##@ Louhi Release

.PHONY: release
release: ensure-builder ## Build and push the multi-arch image for Louhi release.
	@echo "LOUHI: Releasing multi-arch image $(LOUHI_IMAGE)..."
	docker buildx build --platform $(PLATFORMS) \
		--builder $(BUILDER_NAME) \
		--build-arg VERSION_LDFLAGS="$(VERSION_LDFLAGS)" \
		-t $(LOUHI_IMAGE) --push .

##@ Help

.PHONY: help
help: ## Display this help message.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
