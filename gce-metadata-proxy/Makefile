# Makefile for the gce-metadata-proxy component

GCP_PROJECT ?= $(shell gcloud config get-value project)
IMAGE_REPO ?= gcr.io/$(GCP_PROJECT)
IMAGE_NAME ?= gce-metadata-proxy
ALL_PLATFORMS ?= linux/amd64,linux/arm64

IMAGE_TAG ?= $(shell git rev-parse --short HEAD)

GIT_VERSION := $(shell git describe --tags --always --dirty | sed 's|.*/||' | tr -cd '[:alnum:].-')

##@ Development

.PHONY: tidy
tidy: ## Tidy the Go module dependencies.
	go mod tidy

.PHONY: fmt
fmt: ## Format the Go source code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet.
	go vet ./...

.PHONY: test
test: ## Run unit tests.
	go test ./...

.PHONY: build
build: ## Build the binary for the host platform.
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(GIT_VERSION)" -o bin/gce-metadata-proxy .

.PHONY: verify
verify: fmt vet test ## Run all local checks.
	@git diff --quiet -- . || (echo "gce-metadata-proxy: uncommitted changes after fmt" && git --no-pager diff -- . && exit 1)

##@ Images

.PHONY: image
image: ## Build a multi-arch image and push it to IMAGE_REPO as IMAGE_TAG.
	docker buildx build --platform $(ALL_PLATFORMS) \
		--build-arg GIT_VERSION=$(GIT_VERSION) \
		-t $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG) \
		--push .

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make <target>\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-15s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) }' $(MAKEFILE_LIST)
