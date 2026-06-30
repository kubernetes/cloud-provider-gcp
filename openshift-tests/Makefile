GOWORK=off

GOGCFLAGS ?=
LD_FLAGS  ?= -w -s

.PHONY: build
build: cloud-controller-manager-gcp-tests-ext

.PHONY: vendor
vendor:
	GOWORK=$(GOWORK) go mod tidy
	GOWORK=$(GOWORK) go mod vendor

.PHONY: check
check:
	@diff="$$(find . -not \( -wholename './vendor' -o -wholename './vendor/*' \) -prune -o -name '*.go' -exec gofmt -d -s {} + 2>&1 || true)"; \
	if [ -n "$$diff" ]; then \
		echo "$$diff" >&2; \
		exit 1; \
	fi
	GOWORK=$(GOWORK) go vet .

.PHONY: cloud-controller-manager-gcp-tests-ext
cloud-controller-manager-gcp-tests-ext:
	mkdir -p bin
	GOWORK=$(GOWORK) CGO_ENABLED=0 go build -mod=vendor $(GOGCFLAGS) \
	    -o bin/cloud-controller-manager-gcp-tests-ext \
	    -trimpath -ldflags "$(LD_FLAGS)" .
