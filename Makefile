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

all: clean build-all

build-all: clean auth-provider-gcp-linux-amd64 auth-provider-gcp-linux-arm64 auth-provider-gcp-windows-amd64 \
	cloud-controller-manager-linux-amd64 cloud-controller-manager-linux-arm64 \
	gke-gcloud-auth-plugin-linux-amd64 gke-gcloud-auth-plugin-linux-arm64 gke-gcloud-auth-plugin-windows-amd64 gke-gcloud-auth-plugin-windows-arm64	gke-gcloud-auth-plugin-darwin-arm64

cloud-controller-manager-linux-amd64 cloud-controller-manager-linux-arm64: cloud-controller-manager-linux-%:
	mkdir -p release/$(GIT_VERSION)/cloud-controller-manager/linux/$*
	CGO_ENABLED=0 GOOS=linux GOARCH=$* go build $(LDFLAGS) -o release/$(GIT_VERSION)/cloud-controller-manager/linux/$*/cloud-controller-manager k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager

auth-provider-gcp-linux-amd64 auth-provider-gcp-linux-arm64: auth-provider-gcp-linux-%:
	mkdir -p release/$(GIT_VERSION)/auth-provider-gcp/linux/$*
	CGO_ENABLED=0 GOOS=linux GOARCH=$* go build $(LDFLAGS) -o release/$(GIT_VERSION)/auth-provider-gcp/linux/$*/auth-provider-gcp k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp

auth-provider-gcp-windows-amd64:
	mkdir -p release/$(GIT_VERSION)/auth-provider-gcp/windows/amd64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o release/$(GIT_VERSION)/auth-provider-gcp/windows/amd64/auth-provider-gcp.exe k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp

gke-gcloud-auth-plugin-linux-amd64 gke-gcloud-auth-plugin-linux-arm64: gke-gcloud-auth-plugin-linux-%:
	mkdir -p release/$(GIT_VERSION)/gke-gcloud-auth-plugin/linux/$*
	CGO_ENABLED=0 GOOS=linux GOARCH=$* go build $(LDFLAGS) -o release/$(GIT_VERSION)/gke-gcloud-auth-plugin/linux/$*/gke-gcloud-auth-plugin k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin

gke-gcloud-auth-plugin-windows-amd64 gke-gcloud-auth-plugin-windows-arm64: gke-gcloud-auth-plugin-windows-%:
	mkdir -p release/$(GIT_VERSION)/gke-gcloud-auth-plugin/windows/$*
	CGO_ENABLED=0 GOOS=windows GOARCH=$* go build $(LDFLAGS) -o release/$(GIT_VERSION)/gke-gcloud-auth-plugin/windows/$*/gke-gcloud-auth-plugin.exe k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin

gke-gcloud-auth-plugin-darwin-arm64:
	mkdir -p release/$(GIT_VERSION)/gke-gcloud-auth-plugin/darwin/arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o release/$(GIT_VERSION)/gke-gcloud-auth-plugin/darwin/arm64/gke-gcloud-auth-plugin k8s.io/cloud-provider-gcp/cmd/gke-gcloud-auth-plugin


.PHONY: all clean build-all copy-binaries-to-gcs

clean:
	@echo "Cleaning up..."
	@find release/ -type d -mindepth 1 -print0 | xargs -0 rm -rf

release-tars:
	bazel build release:release-tars

copy-binaries-to-gcs: build-all
	gcloud storage cp --recursive release/$(GIT_VERSION) gs://$(BUCKET_NAME)/$(GIT_VERSION)
