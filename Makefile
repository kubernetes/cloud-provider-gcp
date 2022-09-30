include _out/workspace-status.make

_out:
	mkdir -p _out

ifndef MAKE_RESTARTS
_out/workspace-status.make: _out .FORCE
	tools/workspace-status.sh > $@
endif

GOOS ?= linux
GOARCH ?= amd64
GOSRCS = $(wildcard **/*.go)

GCP_CONTROLLER_MANAGER_IMAGE = $(STABLE_IMAGE_REGISTRY)/$(STABLE_IMAGE_REPO)/gcp-controller-manager:$(STABLE_IMAGE_TAG)

_out/gcp-controller-manager.tar: $(GOSRCS)
	docker build \
		-t  $(GCP_CONTROLLER_MANAGER_IMAGE) \
		-f cmd/gcp-controller-manager/Dockerfile  .
	docker save $(GCP_CONTROLLER_MANAGER_IMAGE) > $@

define go_binary_rule

_out/$(GOOS)/$(GOARCH)/$(1): $(SRCS)
	CGO_ENABLED=false GOARCH=$(GOOS) GOARCH=$(GOARCH) go build -o _out/$(GOOS)/$(GOARCH)/$(1) "./cmd/$(1)"

.PHONY: $(1)
$(1): _out/$(GOOS)/$(GOARCH)/$(1)

endef

$(eval $(call go_binary_rule,gke-gcloud-auth-plugin))
$(eval $(call go_binary_rule,gke-exec-auth-plugin))
$(eval $(call go_binary_rule,auth-provider-gcp))

.PHONY: verify
verify:
	./tools/verify-all.sh

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	rm -rf _out

.PHONY: .FORCE
.FORCE:
