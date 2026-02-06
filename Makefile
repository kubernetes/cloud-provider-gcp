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


GIT_VERSION := $(shell git describe --tags --always --dirty | sed 's|.*/||')
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


.PHONY: all clean build-all copy-binaries-to-gcs

clean:
	@echo "Cleaning up..."
	@find release/ -type d -mindepth 1 -print0 | xargs -0 rm -rf

release-tars: build-all
	# Clean up any existing tarballs (but NOT the binaries we just built)
	rm -f release/$(GIT_VERSION)/*.tar.gz
	rm -rf release/$(GIT_VERSION)/manifests
	mkdir -p release/$(GIT_VERSION)
	
	@echo "Determining Kubernetes version..."
	@if [ -n "$(KUBE_VERSION_OVERRIDE)" ]; then \
		echo "Using override version: $(KUBE_VERSION_OVERRIDE)"; \
		echo $(KUBE_VERSION_OVERRIDE) > release/$(GIT_VERSION)/kube-version.txt; \
	else \
		echo "Downloading stable version..."; \
		curl -sL https://dl.k8s.io/release/stable.txt > release/$(GIT_VERSION)/kube-version.txt; \
	fi
	
	# Download upstream tarballs
	mkdir -p release/upstream
	@KUBE_VERSION=$$(cat release/$(GIT_VERSION)/kube-version.txt); \
	echo "Building release for Kubernetes version: $$KUBE_VERSION"; \
	echo "Downloading upstream server tarball..."; \
	curl -L "https://dl.k8s.io/release/$$KUBE_VERSION/kubernetes-server-linux-amd64.tar.gz" -o release/upstream/server.tar.gz; \
	echo "Downloading upstream node tarball..."; \
	curl -L "https://dl.k8s.io/release/$$KUBE_VERSION/kubernetes-node-linux-amd64.tar.gz" -o release/upstream/node.tar.gz

	# Dockerize cloud-controller-manager
	mkdir -p release/$(GIT_VERSION)/docker-build
	cp release/$(GIT_VERSION)/cloud-controller-manager/linux/amd64/cloud-controller-manager release/$(GIT_VERSION)/docker-build/
	echo "FROM registry.k8s.io/build-image/go-runner:v2.4.0-go1.25.6-bookworm.0" > release/$(GIT_VERSION)/docker-build/Dockerfile
	echo "COPY cloud-controller-manager /cloud-controller-manager" >> release/$(GIT_VERSION)/docker-build/Dockerfile
	echo "CMD [\"/cloud-controller-manager\"]" >> release/$(GIT_VERSION)/docker-build/Dockerfile
	
	docker build -t registry.k8s.io/cloud-controller-manager:$(GIT_VERSION) release/$(GIT_VERSION)/docker-build/
	docker save registry.k8s.io/cloud-controller-manager:$(GIT_VERSION) > release/$(GIT_VERSION)/cloud-controller-manager.tar
	echo "$(GIT_VERSION)" > release/$(GIT_VERSION)/cloud-controller-manager.docker_tag
	rm -rf release/$(GIT_VERSION)/docker-build

	# Unpack and Inject Server
	mkdir -p release/temp/server
	tar xzf release/upstream/server.tar.gz -C release/temp/server
	
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
	mkdir -p release/temp/node
	tar xzf release/upstream/node.tar.gz -C release/temp/node
	
	# Inject Auth Provider to Node (needed for kubelet credential provider)
	cp release/$(GIT_VERSION)/auth-provider-gcp/linux/amd64/auth-provider-gcp release/temp/node/kubernetes/node/bin/

	# Repack Node
	tar -czf release/$(GIT_VERSION)/kubernetes-node-linux-amd64.tar.gz -C release/temp/node kubernetes

	# Pack kubernetes-node-windows-amd64.tar.gz (Minimal, per existing logic)
	mkdir -p release/$(GIT_VERSION)/node-windows
	cp -f release/$(GIT_VERSION)/auth-provider-gcp/windows/amd64/auth-provider-gcp.exe release/$(GIT_VERSION)/node-windows/auth-provider-gcp.exe
	tar -czf release/$(GIT_VERSION)/kubernetes-node-windows-amd64.tar.gz \
		--transform 's|release/$(GIT_VERSION)/node-windows/auth-provider-gcp.exe|kubernetes/node/bin/auth-provider-gcp.exe|' \
		release/$(GIT_VERSION)/node-windows/auth-provider-gcp.exe
	rm -rf release/$(GIT_VERSION)/node-windows

	# Pack kubernetes-manifests.tar.gz
	# 1. Download and unpack the UPSTREAM manifests
	mkdir -p release/$(GIT_VERSION)/manifests
	curl -L "https://dl.k8s.io/release/$$(cat release/$(GIT_VERSION)/kube-version.txt)/kubernetes-manifests.tar.gz" -o release/upstream/manifests.tar.gz
	tar xzf release/upstream/manifests.tar.gz -C release/$(GIT_VERSION)/manifests


	# 2. OVERLAY your local changes from the cloud-provider-gcp repo
	# Standard addons should go in the 'addons' directory
	mkdir -p release/$(GIT_VERSION)/manifests/kubernetes/addons
	cp -r cluster/addons/* release/$(GIT_VERSION)/manifests/kubernetes/addons/
	# Additional GCE-specific addons
	cp -r cluster/gce/addons/* release/$(GIT_VERSION)/manifests/kubernetes/addons/ || true

	# GCE specific configs go in 'gci-trusty'
	# Ensure gci-trusty dir exists
	mkdir -p release/$(GIT_VERSION)/manifests/kubernetes/gci-trusty
	cp cluster/gce/manifests/*.manifest release/$(GIT_VERSION)/manifests/kubernetes/gci-trusty/
	# Ignore errors for json/yaml if they don't exist
	cp cluster/gce/manifests/*.json release/$(GIT_VERSION)/manifests/kubernetes/gci-trusty/ || true
	cp cluster/gce/manifests/*.yaml release/$(GIT_VERSION)/manifests/kubernetes/gci-trusty/ || true
	# Substitute variables in manifests
	find release/$(GIT_VERSION)/manifests/kubernetes/gci-trusty -name "*.manifest" -exec sed -i "s|{{pillar\['cloud-controller-manager_docker_tag'\]}}|$(GIT_VERSION)|g" {} +

	# Substitute variables in addons
	find release/$(GIT_VERSION)/manifests/kubernetes/addons -name "*.yaml" -exec sed -i "s|{{ fluentd_gcp_yaml_version }}|$(FLUENTD_GCP_YAML_VERSION)|g" {} +
	find release/$(GIT_VERSION)/manifests/kubernetes/addons -name "*.yaml" -exec sed -i "s|{{ fluentd_gcp_version }}|$(FLUENTD_GCP_VERSION)|g" {} +
	find release/$(GIT_VERSION)/manifests/kubernetes/addons -name "*.yaml" -exec sed -i "s|{{ prometheus_to_sd_prefix }}|$(PROMETHEUS_TO_SD_PREFIX)|g" {} +
	find release/$(GIT_VERSION)/manifests/kubernetes/addons -name "*.yaml" -exec sed -i "s|{{ prometheus_to_sd_endpoint }}|$(PROMETHEUS_TO_SD_ENDPOINT)|g" {} +
	find release/$(GIT_VERSION)/manifests/kubernetes/addons -name "*.yaml" -exec sed -i "s|{{ fluentd_gcp_configmap_name }}|$(FLUENTD_GCP_CONFIGMAP_NAME)|g" {} +
	find release/$(GIT_VERSION)/manifests/kubernetes/addons -name "*.yaml" -exec sed -i "s|{{cloud_controller_manager_docker_tag}}|$(GIT_VERSION)|g" {} +
	
	# Verify critical substitutions
	if grep -qr --include="*.yaml" "{{cloud_controller_manager_docker_tag}}" release/$(GIT_VERSION)/manifests/kubernetes/addons; then \
		echo "Error: Placeholder {{cloud_controller_manager_docker_tag}} still present in addons."; \
		exit 1; \
	fi

	# Include cri-auth-config if present
	cp cluster/gce/manifests/cri-auth-config.yaml release/$(GIT_VERSION)/manifests/kubernetes/gci-trusty/ || true
	
	cp cluster/gce/gci/configure-helper.sh release/$(GIT_VERSION)/manifests/kubernetes/gci-trusty/gci-configure-helper.sh
	cp cluster/gce/gci/configure-helper.sh release/$(GIT_VERSION)/manifests/kubernetes/gci-trusty/configure-helper.sh
	cp cluster/gce/gci/configure-kubeapiserver.sh release/$(GIT_VERSION)/manifests/kubernetes/gci-trusty/configure-kubeapiserver.sh
	if [ -f cluster/gce/gci/gke-internal-configure-helper.sh ]; then \
		cp cluster/gce/gci/gke-internal-configure-helper.sh release/$(GIT_VERSION)/manifests/kubernetes/gci-trusty/; \
	fi
	# 3. Repack the combined manifests
	tar -czf release/$(GIT_VERSION)/kubernetes-manifests.tar.gz \
		-C release/$(GIT_VERSION)/manifests .
	rm -rf release/$(GIT_VERSION)/manifests

	# Cleanup temp
	rm -rf release/temp release/upstream
	
	echo "4. Generating Checksums..."
	shasum -a 1 release/$(GIT_VERSION)/kubernetes-manifests.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-manifests.tar.gz.sha1
	shasum -a 1 release/$(GIT_VERSION)/kubernetes-server-linux-amd64.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-server-linux-amd64.tar.gz.sha1
	shasum -a 1 release/$(GIT_VERSION)/kubernetes-node-linux-amd64.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-node-linux-amd64.tar.gz.sha1
	shasum -a 1 release/$(GIT_VERSION)/kubernetes-node-windows-amd64.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-node-windows-amd64.tar.gz.sha1
	shasum -a 256 release/$(GIT_VERSION)/kubernetes-manifests.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-manifests.tar.gz.sha256
	shasum -a 256 release/$(GIT_VERSION)/kubernetes-server-linux-amd64.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-server-linux-amd64.tar.gz.sha256
	shasum -a 256 release/$(GIT_VERSION)/kubernetes-node-linux-amd64.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-node-linux-amd64.tar.gz.sha256
	shasum -a 256 release/$(GIT_VERSION)/kubernetes-node-windows-amd64.tar.gz | awk '{print $$1}' > release/$(GIT_VERSION)/kubernetes-node-windows-amd64.tar.gz.sha256
	
	echo "Release artifacts generated in release/$(GIT_VERSION)"

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
