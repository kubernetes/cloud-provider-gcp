BINARY_NAME ?= gcp-controller-manager
IMAGE_REPO ?= gcr.io/gke-dev
TAG ?= latest
BINARY_OS ?= linux
BINARY_ARCH ?= amd64

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=$(BINARY_OS) GOARCH=$(BINARY_ARCH) go build -o ./output/$(BINARY_ARCH)/$(BINARY_OS)/$(BINARY_NAME) ./cmd/$(BINARY_NAME)/

.PHONY: clean
clean:
	rm -rf ./output

.PHONY: build-image
build-image: build
	docker build --no-cache -t $(IMAGE_REPO)/$(BINARY_NAME):$(TAG) \
		--build-arg name=$(BINARY_NAME) \
		-f ./Dockerfile \
		./output/$(BINARY_ARCH)/$(BINARY_OS)

.PHONY: push-image
push-image: build-image
	docker push $(IMAGE_REPO)/$(BINARY_NAME):$(TAG)

.PHONY: test
test:
	go test ./...

.PHONY: deps
deps:
	go mod tidy
	go mod vendor
