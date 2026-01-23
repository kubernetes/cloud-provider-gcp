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


GIT_VERSION ?= $(shell git describe --tags --always --dirty | sed 's|.*/||')
GIT_COMMIT ?= $(shell git rev-parse HEAD)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
BUCKET_NAME ?= k8s-staging-cloud-provider-gcp

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
  linux-amd64 linux-arm64 windows-amd64 windows-arm64 darwin-arm64, \
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

.PHONY: gke-gcloud-auth-plugin-linux-amd64 gke-gcloud-auth-plugin-linux-arm64
gke-gcloud-auth-plugin-linux-amd64 gke-gcloud-auth-plugin-linux-arm64: gke-gcloud-auth-plugin-linux-%:
	mkdir -p release/$(GIT_VERSION)/gke-gcloud-auth-plugin/linux/$*
	CGO_ENABLED=0 GOOS=linux GOARCH=$* go build $(LDFLAGS) -o release/$(GIT_VERSION)/gke-gcloud-auth-plugin/linux/$*/gke-gcloud-auth-plugin k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin

.PHONY: gke-gcloud-auth-plugin-windows-amd64 gke-gcloud-auth-plugin-windows-arm64
gke-gcloud-auth-plugin-windows-amd64 gke-gcloud-auth-plugin-windows-arm64: gke-gcloud-auth-plugin-windows-%:
	mkdir -p release/$(GIT_VERSION)/gke-gcloud-auth-plugin/windows/$*
	CGO_ENABLED=0 GOOS=windows GOARCH=$* go build $(LDFLAGS) -o release/$(GIT_VERSION)/gke-gcloud-auth-plugin/windows/$*/gke-gcloud-auth-plugin.exe k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin

.PHONY: gke-gcloud-auth-plugin-darwin-arm64
gke-gcloud-auth-plugin-darwin-arm64:
	mkdir -p release/$(GIT_VERSION)/gke-gcloud-auth-plugin/darwin/arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o release/$(GIT_VERSION)/gke-gcloud-auth-plugin/darwin/arm64/gke-gcloud-auth-plugin k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin
 
.PHONY: copy-binaries-to-gcs
copy-binaries-to-gcs: build-all ## Build and copy binaries to GCS.
	gcloud storage cp --recursive release/$(GIT_VERSION) gs://$(BUCKET_NAME)/$(GIT_VERSION)

.PHONY: release-tars
release-tars: ## Build release tarballs using bazel.
	bazel build release:release-tars

## --------------------------------------
##@ Test
## --------------------------------------

.PHONY: test
test: ## Run unit tests.
	go test -race ./...
	go test -race ./providers/...

.PHONY: test-sh
test-sh: ## Run shell script syntax checks.
	bash -n cluster/common.sh
	bash -n cluster/clientbin.sh
	bash -n cluster/kube-util.sh
