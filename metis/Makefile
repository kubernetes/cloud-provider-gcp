# Makefile for metis component

# Variables for local development, can be overridden by the caller
IMAGE_REGISTRY ?= gcr.io/$(USER)-gke-dev
IMAGE_NAME ?= metis
ALL_PLATFORMS ?= linux/amd64,linux/arm64

# Optional space-separated list of arbitrary extra tags (e.g., pipeline IDs or dev tags)
ADDITIONAL_IMAGE_TAGS ?=

# Version metadata strictly computed by the internal repository state
GIT_VERSION := $(shell git describe --tags --always --dirty)
GIT_COMMIT  := $(shell git rev-parse HEAD)
BUILD_DATE  := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Base tag cleanly stripped of directory paths (natively appends -dirty if uncommitted changes exist)
TAG := $(shell echo $(GIT_VERSION) | sed 's|.*/||')

# Extract short SHA natively via git, explicitly appending -dirty if uncommitted modifications are detected
SHORT_SHA := $(shell git rev-parse --short HEAD)$(shell git diff-index --quiet HEAD -- || echo "-dirty")

# Build the list of all UNIQUE applicable tags and map them to -t arguments
ALL_TAGS := $(sort $(TAG) $(SHORT_SHA) $(ADDITIONAL_IMAGE_TAGS))
DOCKER_TAG_ARGS := $(foreach tag,$(ALL_TAGS),-t $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(tag))

# Standard K8s versioning - injected via linker flags at build time.
VERSION_LDFLAGS := \
  -X 'k8s.io/component-base/version.gitVersion=$(GIT_VERSION)' \
  -X 'k8s.io/component-base/version.gitCommit=$(GIT_COMMIT)' \
  -X 'k8s.io/component-base/version.buildDate=$(BUILD_DATE)'

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

.PHONY: init-buildx
init-buildx: ## Initialize the docker buildx multi-arch builder if it doesn't exist.
	@if ! docker buildx inspect builder > /dev/null 2>&1; then \
		echo "Initializing docker buildx multi-arch builder..."; \
		docker run --privileged --rm tonistiigi/binfmt --install all; \
		docker buildx create --name builder --use; \
	fi

.PHONY: push-image
push-image: init-buildx ## Build and push the multi-arch image.
	@echo "Building and Pushing multi-arch image: $(IMAGE_REGISTRY)/$(IMAGE_NAME)..."
	@echo "Applying tags: $(ALL_TAGS)..."
	docker buildx build --platform $(ALL_PLATFORMS) \
		--build-arg GIT_VERSION=$(GIT_VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		$(DOCKER_TAG_ARGS) --push .

##@ Help

.PHONY: help
help: ## Display this help message.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
