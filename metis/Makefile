# Makefile for metis component

# Variables for local development, can be overridden by the caller
IMAGE_REGISTRY ?= gcr.io/$(USER)-gke-dev
IMAGE_NAME ?= metis
ALL_PLATFORMS ?= linux/amd64,linux/arm64

# Optional space-separated list of arbitrary extra tags (e.g., pipeline IDs or dev tags)
ADDITIONAL_IMAGE_TAGS ?=

# Version metadata strictly computed by the internal repository state
GIT_VERSION := $(shell git describe --tags --always --dirty | sed 's|.*/||')
GIT_COMMIT  := $(shell git rev-parse HEAD)
BUILD_DATE  := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Extract short SHA natively via git, explicitly appending -dirty if uncommitted modifications are detected
SHORT_SHA := $(shell git rev-parse --short HEAD)$(shell git diff-index --quiet HEAD -- || echo "-dirty")

# Build the list of all UNIQUE applicable tags and map them to -t arguments
ALL_TAGS := $(sort $(GIT_VERSION) $(SHORT_SHA) $(ADDITIONAL_IMAGE_TAGS))
DOCKER_TAG_ARGS := $(foreach tag,$(ALL_TAGS),-t $(IMAGE_REGISTRY)/$(IMAGE_NAME):$(tag))

# Standard K8s versioning - injected via linker flags at build time.
VERSION_LDFLAGS := \
  -X 'k8s.io/component-base/version.gitVersion=$(GIT_VERSION)' \
  -X 'k8s.io/component-base/version.gitCommit=$(GIT_COMMIT)' \
  -X 'k8s.io/component-base/version.buildDate=$(BUILD_DATE)'

# GLIBC Compatibility Test Image
#
# Context: Across GKE fleet nodes, the CGO Metis CNI binary executes natively
# inside the host OS space (via the Kubelet) rather than within a container.
# This rigidly binds its link constraints to the host's C standard library.
#
# Baseline: During the GKE release baseline of 1.30.14-gke.2250000, the
# lowest available GLIBC version across the active fleet OS footprints
# (Ubuntu 22.04 LTS and COS Milestone 117) was precisely GLIBC 2.35.
#
# WARNING: Do not link this binary against newer GLIBC symbols. Doing so
# will cause immediate runtime panics when scheduled on baseline fleet nodes.
GLIBC_FLOOR_IMAGE := ubuntu:22.04

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

DOCKER_BUILD_ARGS := \
	--build-arg GIT_VERSION=$(GIT_VERSION) \
	--build-arg GIT_COMMIT=$(GIT_COMMIT) \
	--build-arg BUILD_DATE=$(BUILD_DATE)

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
		$(DOCKER_BUILD_ARGS) \
		$(DOCKER_TAG_ARGS) --push .

.PHONY: build-image-local
build-image-local:
	docker buildx build $(DOCKER_BUILD_ARGS) -t metis-candidate --load .

_tmp/metis-candidate: build-image-local
	@mkdir -p _tmp
	-docker rm -f temp-metis 2>/dev/null
	docker create --name temp-metis metis-candidate
	docker cp temp-metis:/bin/metis _tmp/metis-candidate
	docker rm temp-metis

.PHONY: test-glibc-floor
test-glibc-floor: _tmp/metis-candidate ## Build image and test glibc floor locally in container
	# Verify symbol compatibility by running natively within the configured floor environment.
	docker run --rm -v $(CURDIR)/_tmp/metis-candidate:/metis $(GLIBC_FLOOR_IMAGE) /metis --help

##@ Help

.PHONY: help
help: ## Display this help message.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
