# Binaries
BINS=auth-provider-gcp cloud-controller-manager gke-gcloud-auth-plugin

# Go build flags
GIT_VERSION ?= $(shell git describe --tags --always --dirty)
GIT_COMMIT ?= $(shell git rev-parse HEAD)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -ldflags="\
-X 'k8s.io/component-base/version.gitVersion=$(GIT_VERSION)' \
-X 'k8s.io/component-base/version.gitCommit=$(GIT_COMMIT)' \
-X 'k8s.io/component-base/version.buildDate=$(BUILD_DATE)' \
-s -w"

# Target OS and architecture
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Output directory
OUTPUT_DIR := bin/$(GOOS)_$(GOARCH)

all: build

build: $(addprefix $(OUTPUT_DIR)/,$(BINS))

$(OUTPUT_DIR)/%: cmd/%
	@mkdir -p $(OUTPUT_DIR)
	go build $(LDFLAGS) -o $@ ./cmd/$(*)

build-linux-amd64:
	GOOS=linux GOARCH=amd64 $(MAKE) build

build-windows-amd64:
	GOOS=windows GOARCH=amd64 $(MAKE) build

test-unit:
	go test -v $(shell go list ./... | grep -v /vendor/ | grep -v /e2e/)

test-integration:
	./tools/run-e2e-test.sh

# Image build
REGISTRY ?= gcr.io/k8s-staging-provider-gcp
KO_DOCKER_REPO ?= $(REGISTRY)
KO_LDFLAGS_COMMON = "\
	-X k8s.io/component-base/version.gitVersion=$(GIT_VERSION) \
	-X k8s.io/component-base/version.gitCommit=$(GIT_COMMIT) \
	-X k8s.io/component-base/version.buildDate=$(BUILD_DATE) \
	-X k8s.io/component-base/version.gitTreeState=$(shell if [ -z "`git status --porcelain`" ]; then echo "clean"; else echo "dirty"; fi) \
	"

images: image-cloud-controller-manager image-auth-provider-gcp image-gke-gcloud-auth-plugin

publish-images: publish-image-cloud-controller-manager publish-image-auth-provider-gcp publish-image-gke-gcloud-auth-plugin

image-cloud-controller-manager:
	@echo "Building image $(KO_DOCKER_REPO)/cloud-controller-manager:$(GIT_VERSION)"
	KO_LDFLAGS=$(KO_LDFLAGS_COMMON) ko build --bare --tags=$(GIT_VERSION) --ldflags=$(KO_LDFLAGS) ./cmd/cloud-controller-manager

publish-image-cloud-controller-manager:
	@echo "Publishing image $(KO_DOCKER_REPO)/cloud-controller-manager:$(GIT_VERSION)"
	KO_LDFLAGS=$(KO_LDFLAGS_COMMON) ko publish --tags=$(GIT_VERSION) --ldflags=$(KO_LDFLAGS) ./cmd/cloud-controller-manager

image-auth-provider-gcp:
	@echo "Building image $(KO_DOCKER_REPO)/auth-provider-gcp:$(GIT_VERSION)"
	KO_LDFLAGS=$(KO_LDFLAGS_COMMON) ko build --bare --tags=$(GIT_VERSION) --ldflags=$(KO_LDFLAGS) ./cmd/auth-provider-gcp

publish-image-auth-provider-gcp:
	@echo "Publishing image $(KO_DOCKER_REPO)/auth-provider-gcp:$(GIT_VERSION)"
	KO_LDFLAGS=$(KO_LDFLAGS_COMMON) ko publish --tags=$(GIT_VERSION) --ldflags=$(KO_LDFLAGS) ./cmd/auth-provider-gcp

image-gke-gcloud-auth-plugin:
	@echo "Building image $(KO_DOCKER_REPO)/gke-gcloud-auth-plugin:$(GIT_VERSION)"
	KO_LDFLAGS=$(KO_LDFLAGS_COMMON) ko build --bare --tags=$(GIT_VERSION) --ldflags=$(KO_LDFLAGS) ./cmd/gke-gcloud-auth-plugin

publish-image-gke-gcloud-auth-plugin:
	@echo "Publishing image $(KO_DOCKER_REPO)/gke-gcloud-auth-plugin:$(GIT_VERSION)"
	KO_LDFLAGS=$(KO_LDFLAGS_COMMON) ko publish --tags=$(GIT_VERSION) --ldflags=$(KO_LDFLAGS) ./cmd/gke-gcloud-auth-plugin

release: build-linux-amd64 build-windows-amd64
	@echo "Creating release tarballs..."
	@mkdir -p release
	tar -czvf release/kubernetes-server-linux-amd64.tar.gz -C bin/linux_amd64 cloud-controller-manager auth-provider-gcp
	tar -czvf release/kubernetes-node-windows-amd64.tar.gz -C bin/windows_amd64 auth-provider-gcp
	tar -czvf release/kubernetes-manifests.tar.gz -C deploy cloud-controller-manager.manifest

GCS_BUCKET ?= gs://k8s-staging-provider-gcp-release

publish-release: release
	@echo "Publishing release tarballs to $(GCS_BUCKET)/release/$(GIT_VERSION)"
	gsutil cp release/*.tar.gz $(GCS_BUCKET)/release/$(GIT_VERSION)/

clean:
	@echo "Cleaning up..."
	@rm -rf bin release

.PHONY: all build build-linux-amd64 build-windows-amd64 test-unit test-integration images publish-images image-cloud-controller-manager publish-image-cloud-controller-manager image-auth-provider-gcp publish-image-auth-provider-gcp image-gke-gcloud-auth-plugin publish-image-gke-gcloud-auth-plugin release publish-release clean