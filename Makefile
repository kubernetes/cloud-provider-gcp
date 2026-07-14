# Copyright 2025 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
LOCAL_BIN := $(PROJECT_DIR)/bin

include Makefile-deps.mk
GCP_PROJECT ?= $(shell gcloud config get-value project)

GIT_VERSION := $(shell git describe --tags --always --dirty | sed 's|.*/||' | tr -cd '[:alnum:].-')
GIT_COMMIT := $(shell git rev-parse HEAD)
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
BUCKET_NAME ?= k8s-staging-cloud-provider-gcp

# Addon Versions
FLUENTD_GCP_YAML_VERSION ?= v3.2.0
FLUENTD_GCP_VERSION ?= 1.6.17
PROMETHEUS_TO_SD_PREFIX ?= custom.googleapis.com
PROMETHEUS_TO_SD_ENDPOINT ?= https://monitoring.googleapis.com/
FLUENTD_GCP_CONFIGMAP_NAME ?= fluentd-gcp-config

LDFLAGS := -ldflags="\
-X 'k8s.io/component-base/version.gitVersion=$(GIT_VERSION)' \
-X 'k8s.io/component-base/version.gitCommit=$(GIT_COMMIT)' \
-X 'k8s.io/component-base/version.buildDate=$(BUILD_DATE)' \
-s -w"

AUTH_PROVIDER_GCP := \
  auth-provider-gcp-linux-amd64 \
  auth-provider-gcp-linux-arm64 \
  auth-provider-gcp-windows-amd64

CLOUD_CONTROLLER_MANAGER := \
  cloud-controller-manager-linux-amd64 \
  cloud-controller-manager-linux-arm64

GKE_GCLOUD_AUTH_PLUGIN := \
  $(foreach platform, \
  linux-amd64 linux-arm64 linux-386 windows-amd64 windows-arm64 windows-386 darwin-amd64 darwin-arm64, \
  $(addsuffix $(platform), gke-gcloud-auth-plugin-))

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[.a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


## --------------------------------------
##@ Build
## --------------------------------------

.PHONY: all
all: clean build-all ## Clean and build all binaries.

.PHONY: clean
clean: ## Clean up release directory.
	@echo "Cleaning up..."
	@find release/ -type d -mindepth 1 -print0 | xargs -0 rm -rf

.PHONY: build-all
build-all: $(AUTH_PROVIDER_GCP) $(CLOUD_CONTROLLER_MANAGER) $(GKE_GCLOUD_AUTH_PLUGIN) ## Build all binaries.

.PHONY: cloud-controller-manager-linux-amd64 cloud-controller-manager-linux-arm64
cloud-controller-manager-linux-amd64 cloud-controller-manager-linux-arm64: cloud-controller-manager-linux-%:
	mkdir -p release/$(GIT_VERSION)/cloud-controller-manager/linux/$*
	CGO_ENABLED=0 GOOS=linux GOARCH=$* go build $(LDFLAGS) -o release/$(GIT_VERSION)/cloud-controller-manager/linux/$*/cloud-controller-manager k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager

.PHONY: auth-provider-gcp-linux-amd64 auth-provider-gcp-linux-arm64
auth-provider-gcp-linux-amd64 auth-provider-gcp-linux-arm64: auth-provider-gcp-linux-%:
	mkdir -p release/$(GIT_VERSION)/auth-provider-gcp/linux/$*
	CGO_ENABLED=0 GOOS=linux GOARCH=$* go build $(LDFLAGS) -o release/$(GIT_VERSION)/auth-provider-gcp/linux/$*/auth-provider-gcp k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp

.PHONY: auth-provider-gcp-windows-amd64
auth-provider-gcp-windows-amd64:
	mkdir -p release/$(GIT_VERSION)/auth-provider-gcp/windows/amd64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o release/$(GIT_VERSION)/auth-provider-gcp/windows/amd64/auth-provider-gcp.exe k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp

.PHONY: gke-gcloud-auth-plugin-linux-amd64 gke-gcloud-auth-plugin-linux-arm64 gke-gcloud-auth-plugin-linux-386
gke-gcloud-auth-plugin-linux-amd64 gke-gcloud-auth-plugin-linux-arm64 gke-gcloud-auth-plugin-linux-386: gke-gcloud-auth-plugin-linux-%:
	mkdir -p release/$(GIT_VERSION)/gke-gcloud-auth-plugin/linux/$*
	CGO_ENABLED=0 GOOS=linux GOARCH=$* go build $(LDFLAGS) -o release/$(GIT_VERSION)/gke-gcloud-auth-plugin/linux/$*/gke-gcloud-auth-plugin k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin

.PHONY: gke-gcloud-auth-plugin-windows-amd64 gke-gcloud-auth-plugin-windows-arm64 gke-gcloud-auth-plugin-windows-386
gke-gcloud-auth-plugin-windows-amd64 gke-gcloud-auth-plugin-windows-arm64 gke-gcloud-auth-plugin-windows-386: gke-gcloud-auth-plugin-windows-%:
	mkdir -p release/$(GIT_VERSION)/gke-gcloud-auth-plugin/windows/$*
	CGO_ENABLED=0 GOOS=windows GOARCH=$* go build $(LDFLAGS) -o release/$(GIT_VERSION)/gke-gcloud-auth-plugin/windows/$*/gke-gcloud-auth-plugin.exe k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin

.PHONY: gke-gcloud-auth-plugin-darwin-arm64 gke-gcloud-auth-plugin-darwin-amd64
gke-gcloud-auth-plugin-darwin-arm64 gke-gcloud-auth-plugin-darwin-amd64: gke-gcloud-auth-plugin-darwin-%:
	mkdir -p release/$(GIT_VERSION)/gke-gcloud-auth-plugin/darwin/$*
	CGO_ENABLED=0 GOOS=darwin GOARCH=$* go build $(LDFLAGS) -o release/$(GIT_VERSION)/gke-gcloud-auth-plugin/darwin/$*/gke-gcloud-auth-plugin k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin


## --------------------------------------
##@ Docker
## --------------------------------------

.PHONY: publish
publish: ## Build and push cloud-controller-manager image to IMAGE_REPO.
	@./tools/push-images

.PHONY: bundle
bundle: ## Create a docker image tar bundle for cloud-controller-manager using Dockerfile.
	@echo "Building cloud-controller-manager image using Dockerfile..."
	docker build -t registry.k8s.io/cloud-controller-manager:$(GIT_VERSION) .
	@echo "Creating docker image tar for cloud-controller-manager..."
	mkdir -p release/$(GIT_VERSION)
	docker save registry.k8s.io/cloud-controller-manager:$(GIT_VERSION) > release/$(GIT_VERSION)/cloud-controller-manager.tar
	echo "$(GIT_VERSION)" > release/$(GIT_VERSION)/cloud-controller-manager.docker_tag
	@echo "Bundle created at release/$(GIT_VERSION)/cloud-controller-manager.tar"

.PHONY: clean-builder
clean-builder: ## Remove the docker buildx builder.
	@echo "Removing docker buildx builder..."
	docker buildx rm multiarch-multiplatform-builder || true
	@echo "Docker buildx builder removed."
  
.PHONY: copy-binaries-to-gcs
copy-binaries-to-gcs: build-all ## Build and copy binaries to GCS.
	gcloud storage cp --recursive release/$(GIT_VERSION) gs://$(BUCKET_NAME)/$(GIT_VERSION)

.PHONY: release-tars
release-tars: release-tars-linux-amd64 release-tars-windows-amd64 generate-all-checksums ## Build all release artifacts.

# Helper for release-tars, not intended for direct use.
.PHONY: .ensure-kube-version
.ensure-kube-version:
	@mkdir -p release/$(GIT_VERSION)
	@if [ ! -f release/$(GIT_VERSION)/kube-version.txt ]; then \
		if [ -n "$(KUBE_VERSION_OVERRIDE)" ]; then \
			echo "Using override version: $(KUBE_VERSION_OVERRIDE)"; \
			echo $(KUBE_VERSION_OVERRIDE) > release/$(GIT_VERSION)/kube-version.txt; \
		else \
			echo "Downloading stable version..."; \
			curl -sL https://dl.k8s.io/release/stable.txt > release/$(GIT_VERSION)/kube-version.txt; \
		fi; \
	fi

.PHONY: release-tars-linux-amd64
release-tars-linux-amd64: cloud-controller-manager-linux-amd64 auth-provider-gcp-linux-amd64 bundle .ensure-kube-version ## Build release artifacts for linux/amd64.
	# Download upstream tarballs
	@echo "Packing release tarballs for linux/amd64..."
	mkdir -p release/upstream-linux release/temp/server release/temp/node
	@KUBE_VERSION=$$(cat release/$(GIT_VERSION)/kube-version.txt); \
	echo "Building release for Kubernetes version: $$KUBE_VERSION"; \
	echo "Downloading upstream server tarball..."; \
	curl -L "https://dl.k8s.io/release/$$KUBE_VERSION/kubernetes-server-linux-amd64.tar.gz" -o release/upstream-linux/server.tar.gz; \
	echo "Downloading upstream node tarball..."; \
	curl -L "https://dl.k8s.io/release/$$KUBE_VERSION/kubernetes-node-linux-amd64.tar.gz" -o release/upstream-linux/node.tar.gz

	# Unpack and Inject Server
	tar xzf release/upstream-linux/server.tar.gz -C release/temp/server
	
	# Inject CCM logic
	cp release/$(GIT_VERSION)/cloud-controller-manager.tar release/temp/server/kubernetes/server/bin/
	cp release/$(GIT_VERSION)/cloud-controller-manager.docker_tag release/temp/server/kubernetes/server/bin/
	# Overwrite CCM binary if present
	cp release/$(GIT_VERSION)/cloud-controller-manager/linux/amd64/cloud-controller-manager release/temp/server/kubernetes/server/bin/
	
	# Inject Auth Provider
	cp release/$(GIT_VERSION)/auth-provider-gcp/linux/amd64/auth-provider-gcp release/temp/server/kubernetes/server/bin/

	@echo "Checking contents of release/temp/server/kubernetes/server/bin/ before repack:"
	ls -l release/temp/server/kubernetes/server/bin/

	# Repack Server
	tar -czf release/$(GIT_VERSION)/kubernetes-server-linux-amd64.tar.gz -C release/temp/server kubernetes

	# Unpack and Inject Node
	tar xzf release/upstream-linux/node.tar.gz -C release/temp/node
	
	# Inject Auth Provider to Node (needed for kubelet credential provider)
	cp release/$(GIT_VERSION)/auth-provider-gcp/linux/amd64/auth-provider-gcp release/temp/node/kubernetes/node/bin/

	# Repack Node
	tar -czf release/$(GIT_VERSION)/kubernetes-node-linux-amd64.tar.gz -C release/temp/node kubernetes
	
	rm -rf release/temp release/upstream-linux
	@echo "Linux amd64 release tarballs created in release/$(GIT_VERSION)/"

.PHONY: release-tars-windows-amd64
release-tars-windows-amd64: auth-provider-gcp-windows-amd64 ## Build release artifacts for windows/amd64.
	# Pack kubernetes-node-windows-amd64.tar.gz (Minimal, per existing logic)
	@echo "Packing kubernetes-node-windows-amd64.tar.gz..."
	mkdir -p release/$(GIT_VERSION)/node-windows
	cp -f release/$(GIT_VERSION)/auth-provider-gcp/windows/amd64/auth-provider-gcp.exe release/$(GIT_VERSION)/node-windows/auth-provider-gcp.exe
	tar -czf release/$(GIT_VERSION)/kubernetes-node-windows-amd64.tar.gz \
		--transform 's|release/$(GIT_VERSION)/node-windows/auth-provider-gcp.exe|kubernetes/node/bin/auth-provider-gcp.exe|' \
		release/$(GIT_VERSION)/node-windows/auth-provider-gcp.exe
	rm -rf release/$(GIT_VERSION)/node-windows
	@echo "Windows amd64 release tarball created in release/$(GIT_VERSION)/"

.PHONY: generate-all-checksums
generate-all-checksums: release-tars-linux-amd64 release-tars-windows-amd64 ## Generate checksums for all release artifacts.
	# Final step: generate checksums for all artifacts.
	@echo "Generating Checksums for all release artifacts..."
	@if [ -f release/$(GIT_VERSION)/kubernetes-server-linux-amd64.tar.gz ]; then \
		shasum -a 1 release/$(GIT_VERSION)/kubernetes-server-linux-amd64.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-server-linux-amd64.tar.gz.sha1; \
		shasum -a 256 release/$(GIT_VERSION)/kubernetes-server-linux-amd64.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-server-linux-amd64.tar.gz.sha256; \
	fi
	@if [ -f release/$(GIT_VERSION)/kubernetes-node-windows-amd64.tar.gz ]; then \
		shasum -a 1 release/$(GIT_VERSION)/kubernetes-node-windows-amd64.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-node-windows-amd64.tar.gz.sha1; \
		shasum -a 256 release/$(GIT_VERSION)/kubernetes-node-windows-amd64.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-node-windows-amd64.tar.gz.sha256; \
	fi
	@if [ -f release/$(GIT_VERSION)/kubernetes-node-linux-amd64.tar.gz ]; then \
		shasum -a 1 release/$(GIT_VERSION)/kubernetes-node-linux-amd64.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-node-linux-amd64.tar.gz.sha1; \
		shasum -a 256 release/$(GIT_VERSION)/kubernetes-node-linux-amd64.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-node-linux-amd64.tar.gz.sha256; \
	fi
	@echo "Release artifacts generated in release/$(GIT_VERSION)"

## --------------------------------------
##@ Test
## --------------------------------------

.PHONY: test
test: ## Run unit tests.
	go test -race ./...
	go test -race ./providers/...

## --------------------------------------
##@ Tools
## --------------------------------------

.PHONY: verify
verify: golangci-lint ## Run all verification scripts.
	./tools/verify-all.sh

.PHONY: update-vendor
update-vendor: ## Update vendor directory.
	./tools/update_vendor.sh

.PHONY: update-gofmt
update-gofmt: ## Update gofmt.
	./tools/update-gofmt.sh

.PHONY: update-golang
update-golang: ## Update golang version.
	./dev/tools/update-golang

.PHONY: pin-k8s-deps
pin-k8s-deps: ## Pin Kubernetes dependencies.
	./tools/pin_k8s_deps.sh

.PHONY: push-images
push-images: ## Push images to IMAGE_REPO.
	gcloud auth configure-docker
	IMAGE_REPO=$(IMAGE_REPO) IMAGE_TAG=$(IMAGE_TAG) ./tools/push-images

.PHONY: merge-licenses
merge-licenses: ## Merge licenses from vendor directory.
	./tools/merge_licenses.sh

.PHONY: run-e2e-test
run-e2e-test: ## Run e2e tests.
	./tools/run-e2e-test.sh

.PHONY: verify-up-to-date
verify-up-to-date: ## Verify that the repository is up to date.
	./tools/verify-up-to-date.sh

.PHONY: print-k8s-version
print-k8s-version: ## Print the pinned Kubernetes version.
	@if [ -f ginko-test-package-version.env ]; then cat ginko-test-package-version.env | tr -d '[:space:]'; else curl -sL https://dl.k8s.io/release/stable.txt; fi

## --------------------------------------
##@ Test Cluster E2E
## --------------------------------------

KOPS_CLUSTER_NAME ?= kops-e2e.k8s.local
GCP_LOCATION ?= us-central1
GCP_ZONES ?= $(GCP_LOCATION)-b
IMAGE_REPO ?= gcr.io/$(GCP_PROJECT)
KOPS_STATE_STORE ?= gs://kops-state-$(GCP_PROJECT)
IMAGE_TAG ?= $(shell git rev-parse --short HEAD)

# Defaults for kOps E2E tests
KOPS_FOCUS_REGEX ?= "\[Conformance\]"
KOPS_SKIP_REGEX ?= "\[Serial\]"
TEST_ARGS ?=


.PHONY: test-cluster-simple
test-cluster-simple: ## Run test cluster simple E2E test scenario.
	./e2e/scenarios/kops-simple

.PHONY: test-cluster-tool
test-cluster-tool: $(LOCAL_BIN)/gkops ## Build the test cluster lifecycle tool.

$(LOCAL_BIN)/gkops: tools/kops/main.go tools/kops/pkg/kops/*.go
	@echo "Building kOps lifecycle tool..."
	mkdir -p $(LOCAL_BIN)
	go build -o $(LOCAL_BIN)/gkops tools/kops/main.go

.PHONY: test-cluster-setup
test-cluster-setup: install-test-cluster-deps test-cluster-tool push-images ## Setup environment for test cluster E2E.

.PHONY: test-cluster-up
test-cluster-up: test-cluster-setup ## Provision test cluster.
	PATH=$(LOCAL_BIN):$(PATH) KOPS_STATE_STORE=$(KOPS_STATE_STORE) $(LOCAL_BIN)/gkops up \
		--cluster-name=$(KOPS_CLUSTER_NAME) \
		--gcp-project=$(GCP_PROJECT) \
		--gcp-location=$(GCP_LOCATION) \
		--gcp-zones=$(GCP_ZONES) \
		--state-store=$(KOPS_STATE_STORE) \
		--image-repo=$(IMAGE_REPO) \
		--image-tag=$(IMAGE_TAG)

.PHONY: test-cluster-down
test-cluster-down: test-cluster-tool ## Tear down test cluster.
	PATH=$(LOCAL_BIN):$(PATH) KOPS_STATE_STORE=$(KOPS_STATE_STORE) $(LOCAL_BIN)/gkops down \
		--cluster-name=$(KOPS_CLUSTER_NAME) \
		--gcp-project=$(GCP_PROJECT) \
		--state-store=$(KOPS_STATE_STORE)

.PHONY: test-cluster-e2e-test
test-cluster-e2e-test: test-cluster-tool ## Run E2E tests on test cluster.
	@echo "Running E2E tests on cluster $(KOPS_CLUSTER_NAME)..."
	PATH=$(LOCAL_BIN):$(PATH) KOPS_STATE_STORE=$(KOPS_STATE_STORE) KOPS_CLUSTER_NAME= CLUSTER_NAME= $(LOCAL_BIN)/kubetest2 kops \
		-v=2 \
		--cloud-provider=gce \
		--cluster-name=$(KOPS_CLUSTER_NAME) \
		--kops-binary-path=$(LOCAL_BIN)/kops \
		--gcp-project=$(GCP_PROJECT) \
		--admin-access=$(ADMIN_ACCESS) \
		--test=kops \
		--kubernetes-version=$(K8S_VERSION) \
		-- \
		--parallel=30 \
		--test-package-version="${K8S_VERSION}" \
		--skip-regex="$(KOPS_SKIP_REGEX)" \
		--focus-regex="$(KOPS_FOCUS_REGEX)" \
		$(if $(TEST_ARGS),--test-args="$(TEST_ARGS)")


